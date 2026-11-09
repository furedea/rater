package server_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/furedea/rater/internal/server"
)

func TestParseKeyringMapsEachKeyToItsAnnotator(t *testing.T) {
	t.Parallel()

	alice1, alice2, bob := newSigner(t), newSigner(t), newSigner(t)
	data := strings.Join([]string{
		"# annotators of the pilot study",
		"",
		keyLine("alice", alice1) + " alice@laptop",
		"  alice\t" + authorizedKey(alice2),
		keyLine("bob", bob),
	}, "\n")

	keys, err := server.ParseKeyring([]byte(data))
	require.NoError(t, err)

	for _, tt := range []struct {
		key  gossh.Signer
		want string
	}{{alice1, "alice"}, {alice2, "alice"}, {bob, "bob"}} {
		who, ok := keys.Owner(tt.key.PublicKey())
		require.True(t, ok)
		assert.Equal(t, tt.want, who.String())
	}
}

func TestKeyringDoesNotKnowUnregisteredKeys(t *testing.T) {
	t.Parallel()

	keys, err := server.ParseKeyring([]byte(keyLine("alice", newSigner(t))))
	require.NoError(t, err)

	_, ok := keys.Owner(newSigner(t).PublicKey())

	assert.False(t, ok)
}

func TestParseKeyringRejectsInvalidFiles(t *testing.T) {
	t.Parallel()

	a, b := newSigner(t), newSigner(t)
	tests := []struct {
		name string
		data string
	}{
		{"no keys", "# nobody yet\n"},
		{"name without key", "alice\n"},
		{"invalid name", keyLine("Alice", a)},
		{"malformed key", "alice ssh-ed25519 not-base64"},
		{"key options", "alice " + `from="10.0.0.1" ` + authorizedKey(a)},
		{"same key twice for one annotator", keyLine("alice", a) + "\n" + keyLine("alice", a)},
		{"same key for two annotators", keyLine("alice", a) + "\n" + keyLine("bob", a)},
		{"valid line before a bad one", keyLine("alice", a) + "\n" + keyLine("bob", b) + "\nbob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := server.ParseKeyring([]byte(tt.data))
			assert.ErrorIs(t, err, server.ErrInvalidKeys)
		})
	}
}

func newSigner(t *testing.T) gossh.Signer {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	s, err := gossh.NewSignerFromKey(priv)
	require.NoError(t, err)
	return s
}

func authorizedKey(s gossh.Signer) string {
	return strings.TrimSpace(string(gossh.MarshalAuthorizedKey(s.PublicKey())))
}

func keyLine(name string, s gossh.Signer) string { return name + " " + authorizedKey(s) }
