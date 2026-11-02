// Package server serves the annotation screen over SSH. Each registered key
// names one annotator; a session runs that annotator's own Bubble Tea
// program on their own log, so nobody sees anyone else's work.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	wishtea "charm.land/wish/v2/bubbletea"
	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/sync/errgroup"

	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
	"github.com/furedea/rater/internal/tui"
)

const (
	// handshakeTimeout drops connections that never finish authenticating.
	handshakeTimeout = 30 * time.Second
	// shutdownGrace is how long shutdown waits for clients to disconnect
	// after their sessions quit before it closes the connections.
	shutdownGrace = 5 * time.Second
)

// Config is what a server needs; every part is already validated.
type Config struct {
	TaskDir    string
	Assignment task.Assignment
	Keys       Keyring
	HostKey    gossh.Signer
	// Log receives connection events; nil discards them.
	Log *slog.Logger
}

// Server serves annotation sessions over SSH.
type Server struct {
	dir      string
	plan     task.Assignment
	keys     Keyring
	log      *slog.Logger
	ssh      *ssh.Server
	sessions *sessions
}

var (
	errShuttingDown = errors.New("server is shutting down")
	errUnknownKey   = errors.New("key is not registered")
)

// New configures a server that accepts only the public keys in cfg.Keys,
// gives no shell, and refuses commands and sessions without a terminal.
func New(cfg Config) (*Server, error) {
	s := &Server{
		dir: cfg.TaskDir, plan: cfg.Assignment, keys: cfg.Keys,
		log: cfg.Log, sessions: newSessions(),
	}
	if s.log == nil {
		s.log = slog.New(slog.DiscardHandler)
	}
	srv, err := wish.NewServer(
		func(srv *ssh.Server) error {
			srv.AddHostKey(cfg.HostKey)
			srv.HandshakeTimeout = handshakeTimeout
			srv.SessionRequestCallback = onlyShell
			return nil
		},
		wish.WithPublicKeyAuth(s.registered),
		wish.WithMiddleware(s.annotate, activeterm.Middleware()),
	)
	if err != nil {
		return nil, fmt.Errorf("configure ssh server: %w", err)
	}
	s.ssh = srv
	return s, nil
}

// Serve accepts connections on l until ctx ends, then shuts down gracefully:
// it stops accepting, asks every running session to quit, and returns once
// each session has closed its log.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		err := s.ssh.Serve(l)
		if errors.Is(err, ssh.ErrServerClosed) || ctx.Err() != nil {
			return nil // stopped by shutdown, which closes the listener
		}
		return fmt.Errorf("serve ssh: %w", err)
	})
	g.Go(func() error {
		<-ctx.Done()
		return s.shutdown(l)
	})
	return g.Wait()
}

// shutdown closes l itself as well, because ctx may end before the SSH
// server has registered l, and then the server's shutdown would not close it.
func (s *Server) shutdown(l net.Listener) error {
	closeErr := l.Close()
	s.sessions.close()
	grace, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()
	err := s.ssh.Shutdown(grace)
	if errors.Is(err, context.DeadlineExceeded) {
		err = s.ssh.Close()
	}
	s.sessions.wait()
	// Both close l, so whichever runs second finds it already closed.
	if err := errors.Join(ignoreClosed(closeErr), ignoreClosed(err)); err != nil {
		return fmt.Errorf("shut down ssh: %w", err)
	}
	return nil
}

func ignoreClosed(err error) error {
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

// registered accepts exactly the keys in the keyring. It does not record the
// identity: a client may offer several keys, so the session looks up the key
// that actually authenticated.
func (s *Server) registered(_ ssh.Context, key ssh.PublicKey) bool {
	_, ok := s.keys.Owner(key)
	return ok
}

// onlyShell refuses exec and subsystem requests, so a client gets the
// annotation screen or nothing.
func onlyShell(_ ssh.Session, requestType string) bool { return requestType == "shell" }

func (s *Server) annotate(next ssh.Handler) ssh.Handler {
	return func(sess ssh.Session) {
		if err := s.serveSession(sess); err != nil {
			wish.Errorln(sess, "rater:", err)
			_ = sess.Exit(1) // the client may already be gone; nothing else to tell it
			return
		}
		next(sess)
	}
}

func (s *Server) serveSession(sess ssh.Session) error {
	if !s.sessions.begin() {
		return errShuttingDown
	}
	defer s.sessions.end()
	who, ok := s.keys.Owner(sess.PublicKey())
	if !ok {
		return errUnknownKey
	}
	s.log.Info("session started", "annotator", who.String(), "remote", sess.RemoteAddr().String())
	err := s.runAnnotator(sess, who)
	s.log.Info("session ended", "annotator", who.String(), "error", err)
	return err
}

// runAnnotator opens who's own log and shows who's own session on it. The
// log's exclusive lock makes a second session of the same annotator fail
// here, before anything is shown or written.
func (s *Server) runAnnotator(sess ssh.Session, who task.Annotator) (err error) {
	log, prior, err := store.OpenLog(s.dir, who)
	if errors.Is(err, store.ErrLogInUse) {
		return fmt.Errorf("%s already has an open session; quit it before connecting again", who)
	}
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, log.Close()) }()
	session, err := s.plan.StartSession(who, prior, log)
	if err != nil {
		return fmt.Errorf("resume session: %w", err)
	}
	pty, _, _ := sess.Pty()
	// Later options win, so the capped size replaces the one MakeOptions takes from the client.
	opts := append(wishtea.MakeOptions(sess), tea.WithoutSignalHandler(), tea.WithWindowSize(screenSize(pty.Window)))
	return s.run(sess, tea.NewProgram(tui.NewRemote(session), opts...))
}

// run drives p until it quits, forwarding window resizes and quitting it when
// the client disconnects or the server shuts down.
func (s *Server) run(sess ssh.Session, p *tea.Program) error {
	_, windows, _ := sess.Pty()
	// ctx ends when the client disconnects or, below, when p has quit.
	ctx, cancel := context.WithCancel(sess.Context())
	var wg sync.WaitGroup
	wg.Go(func() { s.steer(ctx, p, windows) })
	_, err := p.Run()
	cancel()
	wg.Wait()
	if err != nil {
		return fmt.Errorf("run annotate screen: %w", err)
	}
	return nil
}

// steer forwards resizes to p and quits it on shutdown or when ctx ends.
// Quitting a program that has already ended does nothing.
func (s *Server) steer(ctx context.Context, p *tea.Program, windows <-chan ssh.Window) {
	closing := s.sessions.closing
	for steering := true; steering; {
		select {
		case <-ctx.Done():
			p.Quit()
			steering = false
		case <-closing:
			p.Quit()
			closing = nil // quit once; keep forwarding until the program ends
		case w, open := <-windows:
			steering = resize(p, w, open)
		}
	}
}

// resize hands p the capped window size. A closed window channel means the
// client closed the session but kept the connection, so p quits and resize
// reports false.
func resize(p *tea.Program, w ssh.Window, open bool) bool {
	if !open {
		p.Quit()
		return false
	}
	width, height := screenSize(w)
	p.Send(tea.WindowSizeMsg{Width: width, Height: height})
	return true
}
