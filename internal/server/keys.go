package server

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	gossh "golang.org/x/crypto/ssh"

	"github.com/furedea/rater/internal/task"
)

// ErrInvalidKeys wraps every validation failure of a keys file.
var ErrInvalidKeys = errors.New("invalid keys file")

// Keyring maps registered SSH public keys to the annotators who own them.
// It is the only source of an annotator's identity on the server.
type Keyring struct {
	owners map[string]task.Annotator // keyed by the wire encoding of the key
}

// ParseKeyring reads a keys file. Each line is an annotator name followed by
// one public key in authorized_keys form ("alice ssh-ed25519 AAAA... comment").
// An annotator may have several lines. Blank lines and lines starting with #
// are ignored. Key options such as from="..." are rejected rather than
// silently not enforced, and a key may appear only once so that it names
// exactly one annotator.
func ParseKeyring(data []byte) (Keyring, error) {
	owners := map[string]task.Annotator{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		if err := addKeyLine(owners, text); err != nil {
			return Keyring{}, fmt.Errorf("%w: line %d: %w", ErrInvalidKeys, line, err)
		}
	}
	if err := sc.Err(); err != nil {
		return Keyring{}, fmt.Errorf("scan keys: %w", err)
	}
	if len(owners) == 0 {
		return Keyring{}, fmt.Errorf("%w: no keys", ErrInvalidKeys)
	}
	return Keyring{owners: owners}, nil
}

// LoadKeyring reads and parses the keys file at path.
func LoadKeyring(path string) (Keyring, error) {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return Keyring{}, fmt.Errorf("open keys dir: %w", err)
	}
	data, readErr := root.ReadFile(filepath.Base(path))
	closeErr := root.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return Keyring{}, fmt.Errorf("read keys %s: %w", path, err)
	}
	k, err := ParseKeyring(data)
	if err != nil {
		return Keyring{}, fmt.Errorf("keys %s: %w", path, err)
	}
	return k, nil
}

// Owner returns the annotator a registered key belongs to.
func (k Keyring) Owner(key gossh.PublicKey) (task.Annotator, bool) {
	who, ok := k.owners[string(key.Marshal())]
	return who, ok
}

func addKeyLine(owners map[string]task.Annotator, text string) error {
	end := strings.IndexFunc(text, unicode.IsSpace)
	if end < 0 {
		return fmt.Errorf("annotator %q has no key", text)
	}
	name, authorized := text[:end], text[end:]
	who, err := task.NewAnnotator(name)
	if err != nil {
		return err
	}
	key, _, options, _, err := gossh.ParseAuthorizedKey([]byte(strings.TrimSpace(authorized)))
	if err != nil {
		return fmt.Errorf("key of %s: %w", who, err)
	}
	if len(options) > 0 {
		return fmt.Errorf("key of %s: options are not supported: %s", who, strings.Join(options, ","))
	}
	wire := string(key.Marshal())
	if prev, dup := owners[wire]; dup {
		return fmt.Errorf("key of %s is already registered to %s", who, prev)
	}
	owners[wire] = who
	return nil
}
