package task

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Key is one keystroke in the canonical form of Bubble Tea's KeyPressMsg.String:
// modifiers in the order ctrl, alt, shift, meta, hyper, super; a shifted letter
// without other modifiers written as the upper-case letter; named keys in lower case.
// Spellings that differ only in these respects parse to the same Key.
type Key struct{ name string }

var modifierOrder = []string{"ctrl", "alt", "shift", "meta", "hyper", "super"}

var modifierAliases = map[string]string{"control": "ctrl", "option": "alt", "cmd": "super"}

var keyAliases = map[string]string{
	"escape": "esc", "return": "enter", "pageup": "pgup", "pagedown": "pgdown", " ": "space",
}

// ParseKey reads a key written in Bubble Tea notation, such as "down", "ctrl+j", or "S".
func ParseKey(s string) (Key, error) {
	if s != " " && (s == "" || strings.TrimSpace(s) != s) {
		return Key{}, fmt.Errorf("unusable key %q", s)
	}
	mods, base := splitKey(s)
	if base == "" {
		return Key{}, fmt.Errorf("key %q names no key after its modifiers", s)
	}
	held, err := parseModifiers(mods)
	if err != nil {
		return Key{}, fmt.Errorf("key %q: %w", s, err)
	}
	return Key{name: canonical(held, base)}, nil
}

// KeyOfRune returns the key that types r.
func KeyOfRune(r rune) Key {
	k, err := ParseKey(string(r))
	if err != nil {
		return Key{name: string(r)}
	}
	return k
}

// String returns the canonical notation.
func (k Key) String() string { return k.name }

// Rune returns the single character the key types, if it types one.
func (k Key) Rune() (rune, bool) {
	r, size := utf8.DecodeRuneInString(k.name)
	if size == 0 || size != len(k.name) {
		return 0, false
	}
	return r, true
}

// splitKey separates "ctrl+alt+x" into its modifiers and the base key; a
// trailing "+" is the plus key itself.
func splitKey(s string) (mods []string, base string) {
	if strings.HasSuffix(s, "++") || s == "+" {
		return splitMods(strings.TrimSuffix(s, "+")), "+"
	}
	parts := strings.Split(s, "+")
	return parts[:len(parts)-1], parts[len(parts)-1]
}

func splitMods(s string) []string {
	s = strings.TrimSuffix(s, "+")
	if s == "" {
		return nil
	}
	return strings.Split(s, "+")
}

func parseModifiers(mods []string) (map[string]bool, error) {
	held := map[string]bool{}
	for _, m := range mods {
		name := strings.ToLower(m)
		if alias, ok := modifierAliases[name]; ok {
			name = alias
		}
		if !slices.Contains(modifierOrder, name) {
			return nil, fmt.Errorf("unknown modifier %q", m)
		}
		if held[name] {
			return nil, fmt.Errorf("modifier %q given twice", m)
		}
		held[name] = true
	}
	return held, nil
}

func canonical(held map[string]bool, base string) string {
	base = canonicalBase(held, base)
	var b strings.Builder
	for _, m := range modifierOrder {
		if held[m] {
			b.WriteString(m + "+")
		}
	}
	b.WriteString(base)
	return b.String()
}

// canonicalBase folds case: a letter typed with shift alone is its upper case
// (the text Bubble Tea reports), and an upper-case letter with other modifiers
// is the lower-case letter with shift.
func canonicalBase(held map[string]bool, base string) string {
	r, size := utf8.DecodeRuneInString(base)
	if size != len(base) {
		lower := strings.ToLower(base)
		if alias, ok := keyAliases[lower]; ok {
			return alias
		}
		return lower
	}
	if alias, ok := keyAliases[base]; ok {
		return alias
	}
	if !unicode.IsLetter(r) {
		return base
	}
	onlyShift := held["shift"] && len(held) == 1
	switch {
	case onlyShift:
		delete(held, "shift")
		return string(unicode.ToUpper(r))
	case len(held) > 0 && unicode.IsUpper(r):
		held["shift"] = true
		return string(unicode.ToLower(r))
	}
	return base
}
