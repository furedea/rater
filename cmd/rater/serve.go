package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	gossh "golang.org/x/crypto/ssh"

	"github.com/furedea/rater/internal/server"
	"github.com/furedea/rater/internal/store"
)

const serveHelp = `usage: rater serve --task <dir> --listen <addr> --keys <file> [--host-key <path>]

Serves the annotation screen of the task over SSH. Annotators connect with
"ssh -p <port> <host>" and get the screen, never a shell; commands, port
forwarding, and sessions without a terminal are refused.

Keys file: one line per key, an annotator name and an authorized_keys entry:

  # name   key
  alice    ssh-ed25519 AAAAC3Nza... alice@laptop
  alice    ssh-ed25519 AAAAC3Nza... alice@desktop
  bob      ssh-rsa AAAAB3Nza...

Only public-key logins with these keys are accepted. The key decides who is
annotating; the SSH user name is ignored. Each annotator sees only their own
labels, notes, and progress. An annotator has one session at a time; a second
connection is refused until the first quits.

Host key: --host-key, by default <dir>/host_ed25519, is created with mode 0600
on first start and reused afterwards; keep it out of version control.

Assignment: an optional <dir>/assignment.yaml limits the items each annotator
sees (annotate uses it too); annotators it does not name see every item:

  split:                       # deal items round-robin to a group
    annotators: [alice, bob, carol]
    copies: 2                  # each item goes to 2 of them, for agreement
  items:                       # or list items for one annotator
    dave: [item-1, item-7]

SIGINT or SIGTERM quits every session and stops the server; every label is
already on disk when it is made. adjudicate is not served; run it locally.
`

// serveOptions are the flags of rater serve.
type serveOptions struct {
	taskDir     string
	listen      string
	keysPath    string
	hostKeyPath string
}

const defaultHostKey = "host_ed25519"

func runServe(args []string, out io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, restore the default handling so that a second
	// one kills the server if a graceful shutdown hangs.
	context.AfterFunc(ctx, stop)
	return serve(ctx, args, out)
}

// serve validates everything before it listens, then serves until ctx ends.
func serve(ctx context.Context, args []string, out io.Writer) error {
	opts, err := parseServeArgs(args)
	if errors.Is(err, flag.ErrHelp) {
		_, err := fmt.Fprint(out, serveHelp)
		return err
	}
	if err != nil {
		return err
	}
	ws, err := store.LoadWorkspace(opts.taskDir)
	if err != nil {
		return err
	}
	plan, err := store.LoadAssignment(opts.taskDir, ws)
	if err != nil {
		return err
	}
	keys, err := server.LoadKeyring(opts.keysPath)
	if err != nil {
		return err
	}
	hostKey, err := server.LoadHostKey(opts.hostKeyPath)
	if err != nil {
		return err
	}
	srv, err := server.New(server.Config{
		TaskDir: opts.taskDir, Assignment: plan, Keys: keys, HostKey: hostKey,
		Log: slog.New(slog.NewTextHandler(out, nil)),
	})
	if err != nil {
		return err
	}
	l, err := new(net.ListenConfig).Listen(ctx, "tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", opts.listen, err)
	}
	if _, err := fmt.Fprintf(out, "serving %s on %s (host key %s)\n",
		ws.Task.Name(), l.Addr(), gossh.FingerprintSHA256(hostKey.PublicKey())); err != nil {
		return errors.Join(err, l.Close())
	}
	return srv.Serve(ctx, l)
}

func parseServeArgs(args []string) (serveOptions, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	taskDir := fs.String("task", "", "task directory containing task.yaml and items.jsonl")
	listen := fs.String("listen", "", "address to listen on, such as :2222")
	keys := fs.String("keys", "", "keys file mapping annotators to SSH public keys")
	hostKey := fs.String("host-key", "", "host key path (default <task>/"+defaultHostKey+")")
	if err := fs.Parse(args); err != nil {
		return serveOptions{}, fmt.Errorf("parse flags: %w", err)
	}
	for _, required := range []struct{ name, value string }{{"task", *taskDir}, {"listen", *listen}, {"keys", *keys}} {
		if required.value == "" {
			return serveOptions{}, fmt.Errorf("--%s is required", required.name)
		}
	}
	opts := serveOptions{taskDir: *taskDir, listen: *listen, keysPath: *keys, hostKeyPath: *hostKey}
	if opts.hostKeyPath == "" {
		opts.hostKeyPath = filepath.Join(*taskDir, defaultHostKey)
	}
	return opts, nil
}
