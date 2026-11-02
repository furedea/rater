package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	gossh "golang.org/x/crypto/ssh"
)

// hostKeyPerm keeps the host key private to the user running the server.
const hostKeyPerm = 0o600

// ErrHostKeyExposed means the host key file is readable or writable by
// someone other than its owner, so it may have leaked.
var ErrHostKeyExposed = errors.New("host key is accessible to group or others")

// LoadHostKey reads the server's private host key from path. On first start,
// when path does not exist, it generates an Ed25519 key and writes it there
// with owner-only permissions, so clients see the same host key across
// restarts. Like sshd, it refuses a key file that others can access.
func LoadHostKey(path string) (gossh.Signer, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, fmt.Errorf("open host key dir: %w", err)
	}
	defer func() { _ = root.Close() }() // read-only use; nothing to flush
	name := filepath.Base(path)
	data, err := readHostKey(root, name)
	if errors.Is(err, fs.ErrNotExist) {
		data, err = createHostKey(root, name)
	}
	if err != nil {
		return nil, fmt.Errorf("host key %s: %w", path, err)
	}
	signer, err := gossh.ParsePrivateKey(data)
	if err != nil {
		return nil, fmt.Errorf("parse host key %s: %w", path, err)
	}
	return signer, nil
}

func readHostKey(root *os.Root, name string) ([]byte, error) {
	info, err := root.Stat(name)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}
	if info.Mode().Perm()&^hostKeyPerm != 0 {
		return nil, fmt.Errorf("%w: mode %v, want %v", ErrHostKeyExposed, info.Mode().Perm(), os.FileMode(hostKeyPerm))
	}
	data, err := root.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	return data, nil
}

// createHostKey writes a new key with O_EXCL, so it never overwrites a key
// that appeared since the existence check.
func createHostKey(root *os.Root, name string) ([]byte, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}
	block, err := gossh.MarshalPrivateKey(priv, "rater host key")
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	data := pem.EncodeToMemory(block)
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, hostKeyPerm)
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	if err := errors.Join(writeErr, syncErr, f.Close()); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}
	return data, nil
}
