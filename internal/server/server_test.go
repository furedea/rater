package server_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/furedea/rater/internal/server"
	"github.com/furedea/rater/internal/store"
	"github.com/furedea/rater/internal/task"
)

const (
	taskYAML = `name: pairs
fields:
  - name: a
labels:
  - {key: s, name: same}
  - {key: d, name: distinct}
`
	itemsJSONL = `{"id":"1","a":"first"}
{"id":"2","a":"second"}
{"id":"3","a":"third"}
`
	waitLimit = 10 * time.Second
	keyLabel  = "s"
	keyQuit   = "\x03"
	keyPrev   = "\x1b[D"
)

func TestServeRejectsAnUnregisteredKey(t *testing.T) {
	h := startServer(t, map[string]gossh.Signer{"alice": newSigner(t)})

	_, err := h.dial("alice", newSigner(t))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to authenticate")
}

func TestServeRejectsPasswordLogins(t *testing.T) {
	h := startServer(t, map[string]gossh.Signer{"alice": newSigner(t)})

	_, err := gossh.Dial("tcp", h.addr, &gossh.ClientConfig{
		User:            "alice",
		Auth:            []gossh.AuthMethod{gossh.Password("alice")},
		HostKeyCallback: gossh.FixedHostKey(h.hostKey),
		Timeout:         waitLimit,
	})

	require.Error(t, err)
}

func TestServeRecordsUnderTheAnnotatorOfTheKeyNotTheLoginName(t *testing.T) {
	alice := newSigner(t)
	h := startServer(t, map[string]gossh.Signer{"alice": alice, "bob": newSigner(t)})
	term := openTerminal(t, h.mustDial(t, "bob", alice))
	term.waitFor(t, "first")
	term.press(t, keyLabel+keyQuit)
	require.NoError(t, term.wait(t))

	records := readRecords(t, h.dir, "alice")
	require.Len(t, records, 1)
	assert.Equal(t, task.Annotation{ItemID: "1", Annotator: "alice", Action: task.ActionLabel, Label: "same"}, withoutTime(records[0]))
	assert.NoFileExists(t, filepath.Join(h.dir, "annotations", "bob.jsonl"))
}

func TestServeRunsDifferentAnnotatorsConcurrentlyAndBlind(t *testing.T) {
	alice, bob := newSigner(t), newSigner(t)
	h := startServer(t, map[string]gossh.Signer{"alice": alice, "bob": bob})
	aliceTerm := openTerminal(t, h.mustDial(t, "alice", alice))
	bobTerm := openTerminal(t, h.mustDial(t, "bob", bob))
	aliceTerm.waitFor(t, "first")
	bobTerm.waitFor(t, "first")

	aliceTerm.press(t, keyLabel)
	bobTerm.press(t, "d")
	aliceTerm.waitFor(t, "second")
	bobTerm.waitFor(t, "second")
	aliceTerm.press(t, keyPrev)
	bobTerm.press(t, keyPrev)
	aliceTerm.waitForSqueezed(t, "labeledassame")
	bobTerm.waitForSqueezed(t, "labeledasdistinct")
	aliceTerm.press(t, keyQuit)
	bobTerm.press(t, keyQuit)
	require.NoError(t, aliceTerm.wait(t))
	require.NoError(t, bobTerm.wait(t))

	assert.Equal(t, "same", readRecords(t, h.dir, "alice")[0].Label)
	assert.Equal(t, "distinct", readRecords(t, h.dir, "bob")[0].Label)
	assert.NotContains(t, squeeze(bobTerm.text()), "labeledassame", "bob never sees alice's label")
	assert.NotContains(t, squeeze(aliceTerm.text()), "labeledasdistinct", "alice never sees bob's label")
}

func TestServeStartsEachAnnotatorWithOnlyTheirOwnProgress(t *testing.T) {
	alice, bob := newSigner(t), newSigner(t)
	h := startServer(t, map[string]gossh.Signer{"alice": alice, "bob": bob})
	aliceTerm := openTerminal(t, h.mustDial(t, "alice", alice))
	aliceTerm.waitFor(t, "first")
	aliceTerm.press(t, keyLabel+keyLabel+keyQuit)
	require.NoError(t, aliceTerm.wait(t))

	bobTerm := openTerminal(t, h.mustDial(t, "bob", bob))

	bobTerm.waitFor(t, "first")
	bobTerm.press(t, keyQuit)
	require.NoError(t, bobTerm.wait(t))
	assert.Contains(t, squeeze(bobTerm.text()), "labeled0", "bob's progress ignores alice's two labels")
}

func TestServeRefusesASecondSessionOfTheSameAnnotator(t *testing.T) {
	alice := newSigner(t)
	h := startServer(t, map[string]gossh.Signer{"alice": alice})
	first := openTerminal(t, h.mustDial(t, "alice", alice))
	first.waitFor(t, "first")

	second := openTerminal(t, h.mustDial(t, "alice", alice))
	err := second.wait(t)

	var exit *gossh.ExitError
	require.ErrorAs(t, err, &exit)
	assert.Equal(t, 1, exit.ExitStatus())
	assert.Contains(t, second.errText(), "alice already has an open session")
	first.press(t, keyLabel+keyQuit)
	require.NoError(t, first.wait(t))
	assert.Len(t, readRecords(t, h.dir, "alice"), 1, "the first session keeps working")
}

func TestServeLetsAnAnnotatorReconnectAfterADroppedConnection(t *testing.T) {
	alice := newSigner(t)
	h := startServer(t, map[string]gossh.Signer{"alice": alice})
	client := h.mustDial(t, "alice", alice)
	first := openTerminal(t, client)
	first.waitFor(t, "first")
	first.press(t, keyLabel)
	first.waitFor(t, "second")

	require.NoError(t, client.Close())

	require.Eventually(t, func() bool {
		l, _, err := store.OpenLog(h.dir, mustAnnotator(t, "alice"))
		if err != nil {
			return false
		}
		return l.Close() == nil
	}, waitLimit, 10*time.Millisecond, "the dropped session releases alice's log")
	again := openTerminal(t, h.mustDial(t, "alice", alice))
	again.waitFor(t, "second")
	again.press(t, keyQuit)
	require.NoError(t, again.wait(t))
}

func TestServeGivesNoCommandSubsystemOrForwarding(t *testing.T) {
	alice := newSigner(t)
	h := startServer(t, map[string]gossh.Signer{"alice": alice})
	client := h.mustDial(t, "alice", alice)
	newSession := func() *gossh.Session {
		sess, err := client.NewSession()
		require.NoError(t, err)
		t.Cleanup(func() { _ = sess.Close() })
		return sess
	}

	require.Error(t, newSession().Run("cat /etc/passwd"), "exec")
	require.Error(t, newSession().RequestSubsystem("sftp"), "subsystem")
	_, err := client.Dial("tcp", h.addr)
	require.Error(t, err, "local port forwarding")
	_, err = client.Listen("tcp", "127.0.0.1:0")
	require.Error(t, err, "remote port forwarding")
	assert.NoFileExists(t, filepath.Join(h.dir, "annotations", "alice.jsonl"))
}

func TestServeRequiresATerminal(t *testing.T) {
	alice := newSigner(t)
	h := startServer(t, map[string]gossh.Signer{"alice": alice})
	sess, err := h.mustDial(t, "alice", alice).NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })

	err = sess.Shell()
	require.NoError(t, err)

	var exit *gossh.ExitError
	require.ErrorAs(t, sess.Wait(), &exit)
	assert.Equal(t, 1, exit.ExitStatus())
}

func TestServeShowsOnlyAssignedItems(t *testing.T) {
	alice := newSigner(t)
	h := startServerWith(t, map[string]gossh.Signer{"alice": alice}, "items:\n  alice: [\"2\"]\n")
	term := openTerminal(t, h.mustDial(t, "alice", alice))

	term.waitFor(t, "second")
	term.press(t, keyLabel+keyQuit)
	require.NoError(t, term.wait(t))

	assert.Equal(t, "2", readRecords(t, h.dir, "alice")[0].ItemID)
	assert.NotContains(t, term.text(), "first")
}

func TestServeShutsDownGracefullyWhenTheContextEnds(t *testing.T) {
	alice := newSigner(t)
	h := startServer(t, map[string]gossh.Signer{"alice": alice})
	client := h.mustDial(t, "alice", alice)
	term := openTerminal(t, client)
	term.waitFor(t, "first")
	term.press(t, keyLabel)
	term.waitFor(t, "second")

	stopped := make(chan error, 1)
	go func() { stopped <- h.stop() }()

	require.NoError(t, term.wait(t), "the session ends cleanly")
	require.NoError(t, client.Close())
	require.NoError(t, <-stopped)
	assert.Len(t, readRecords(t, h.dir, "alice"), 1)
	l, _, err := store.OpenLog(h.dir, mustAnnotator(t, "alice"))
	require.NoError(t, err, "the session released its log")
	require.NoError(t, l.Close())
	_, err = net.DialTimeout("tcp", h.addr, time.Second)
	assert.Error(t, err, "the listener is closed")
}

type harness struct {
	addr    string
	dir     string
	hostKey gossh.PublicKey
	stop    func() error
}

func startServer(t *testing.T, signers map[string]gossh.Signer) *harness {
	t.Helper()
	return startServerWith(t, signers, "")
}

func startServerWith(t *testing.T, signers map[string]gossh.Signer, assignment string) *harness {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "task.yaml"), taskYAML)
	writeFile(t, filepath.Join(dir, "items.jsonl"), itemsJSONL)
	if assignment != "" {
		writeFile(t, filepath.Join(dir, "assignment.yaml"), assignment)
	}
	ws, err := store.LoadWorkspace(dir)
	require.NoError(t, err)
	plan, err := store.LoadAssignment(dir, ws)
	require.NoError(t, err)
	var lines []string
	for name, s := range signers {
		lines = append(lines, keyLine(name, s))
	}
	keys, err := server.ParseKeyring([]byte(strings.Join(lines, "\n")))
	require.NoError(t, err)
	hostKey, err := server.LoadHostKey(filepath.Join(t.TempDir(), "host_ed25519"))
	require.NoError(t, err)
	srv, err := server.New(server.Config{TaskDir: dir, Assignment: plan, Keys: keys, HostKey: hostKey})
	require.NoError(t, err)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ctx, l) }()
	stop := sync.OnceValue(func() error {
		cancel()
		select {
		case err := <-served:
			return err
		case <-time.After(waitLimit):
			return errors.New("server did not stop")
		}
	})
	t.Cleanup(func() { assert.NoError(t, stop()) })
	return &harness{addr: l.Addr().String(), dir: dir, hostKey: hostKey.PublicKey(), stop: stop}
}

func (h *harness) dial(user string, key gossh.Signer) (*gossh.Client, error) {
	return gossh.Dial("tcp", h.addr, &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(key)},
		HostKeyCallback: gossh.FixedHostKey(h.hostKey),
		Timeout:         waitLimit,
	})
}

func (h *harness) mustDial(t *testing.T, user string, key gossh.Signer) *gossh.Client {
	t.Helper()
	c, err := h.dial(user, key)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// terminal is a client session with a PTY that records what the server shows.
type terminal struct {
	sess   *gossh.Session
	stdin  io.Writer
	out    *syncBuffer
	errOut *syncBuffer
}

func openTerminal(t *testing.T, c *gossh.Client) *terminal {
	t.Helper()
	sess, err := c.NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })
	stdin, err := sess.StdinPipe()
	require.NoError(t, err)
	term := &terminal{sess: sess, stdin: stdin, out: &syncBuffer{}, errOut: &syncBuffer{}}
	sess.Stdout, sess.Stderr = term.out, term.errOut
	require.NoError(t, sess.RequestPty("xterm-256color", 24, 100, gossh.TerminalModes{}))
	require.NoError(t, sess.Shell())
	return term
}

func (term *terminal) press(t *testing.T, keys string) {
	t.Helper()
	_, err := io.WriteString(term.stdin, keys)
	require.NoError(t, err)
}

func (term *terminal) waitFor(t *testing.T, text string) {
	t.Helper()
	require.Eventually(t, func() bool { return strings.Contains(term.text(), text) },
		waitLimit, 10*time.Millisecond, "screen never showed %q; got %q", text, term.text())
}

// waitForSqueezed waits for text, compared with all whitespace removed.
func (term *terminal) waitForSqueezed(t *testing.T, text string) {
	t.Helper()
	require.Eventually(t, func() bool { return strings.Contains(squeeze(term.text()), text) },
		waitLimit, 10*time.Millisecond, "screen never showed %q; got %q", text, term.text())
}

func (term *terminal) wait(t *testing.T) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- term.sess.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(waitLimit):
		t.Fatal("session did not end")
		return nil
	}
}

func (term *terminal) text() string    { return ansi.Strip(term.out.String()) }
func (term *terminal) errText() string { return term.errOut.String() }

// syncBuffer is a bytes.Buffer safe for the SSH session's copy goroutine and the test.
type syncBuffer struct {
	mu  sync.Mutex // guards buf
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func readRecords(t *testing.T, dir, name string) []task.Annotation {
	t.Helper()
	l, err := store.ReadLog(dir, mustAnnotator(t, name))
	require.NoError(t, err)
	return l.Records
}

func withoutTime(a task.Annotation) task.Annotation {
	a.At = time.Time{}
	return a
}

func mustAnnotator(t *testing.T, name string) task.Annotator {
	t.Helper()
	who, err := task.NewAnnotator(name)
	require.NoError(t, err)
	return who
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// squeeze drops whitespace, which the renderer may replace with cursor moves.
func squeeze(s string) string { return strings.Join(strings.Fields(s), "") }
