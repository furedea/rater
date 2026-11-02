package task

import (
	"fmt"
)

// Fixed control keys of the annotate screen, in canonical Key notation.
// Bindings may not reuse them.
const (
	KeyQuit      = "esc"
	KeyInterrupt = "ctrl+c"
	KeyHold      = "tab"
	KeyPrev      = "left"
	KeyNext      = "right"
	KeyUndo      = "ctrl+u"
	KeyNote      = "ctrl+n"
)

// Binding is a view action whose key a task file may change.
type Binding int

// Bindings.
const (
	BindingScrollUp Binding = iota
	BindingScrollDown
	BindingPageUp
	BindingPageDown
	BindingNextHunk
	BindingPrevHunk
	bindingCount
)

func (b Binding) String() string {
	switch b {
	case BindingScrollUp:
		return "scroll_up"
	case BindingScrollDown:
		return "scroll_down"
	case BindingPageUp:
		return "page_up"
	case BindingPageDown:
		return "page_down"
	case BindingNextHunk:
		return "next_hunk"
	case BindingPrevHunk:
		return "prev_hunk"
	case bindingCount:
	}
	return fmt.Sprintf("Binding(%d)", int(b))
}

// Bindings lists every binding in a stable order.
func Bindings() []Binding {
	all := make([]Binding, 0, bindingCount)
	for b := range bindingCount {
		all = append(all, b)
	}
	return all
}

// ParseBinding is the inverse of Binding.String.
func ParseBinding(s string) (Binding, error) {
	for _, b := range Bindings() {
		if b.String() == s {
			return b, nil
		}
	}
	return 0, fmt.Errorf("unknown key binding %q", s)
}

// Keys maps each binding to the key that triggers it.
type Keys [bindingCount]Key

var defaultKeys = map[Binding]string{
	BindingScrollUp:   "up",
	BindingScrollDown: "down",
	BindingPageUp:     "pgup",
	BindingPageDown:   "pgdown",
	BindingNextHunk:   "]",
	BindingPrevHunk:   "[",
}

var controlKeys = []string{KeyQuit, KeyInterrupt, KeyHold, KeyPrev, KeyNext, KeyUndo, KeyNote}

// newKeys applies overrides to the defaults and rejects keys that another
// binding, a control key, or a label already uses. Keys are compared in their
// canonical form, so "shift+s" collides with a label on "S".
func newKeys(overrides map[Binding]string, labels []Label) (Keys, error) {
	owner := map[Key]string{}
	for _, k := range controlKeys {
		owner[Key{name: k}] = "reserved"
	}
	for _, l := range labels {
		owner[KeyOfRune(l.Key)] = fmt.Sprintf("label %q", l.Name)
	}
	var keys Keys
	for _, b := range Bindings() {
		written, ok := overrides[b]
		if !ok {
			written = defaultKeys[b]
		}
		key, err := ParseKey(written)
		if err != nil {
			return Keys{}, invalidTask("key binding %s: %v", b, err)
		}
		if err := claimKey(owner, b, key); err != nil {
			return Keys{}, err
		}
		keys[b] = key
	}
	return keys, nil
}

func claimKey(owner map[Key]string, b Binding, key Key) error {
	switch prior, taken := owner[key]; {
	case !taken:
		owner[key] = "binding"
		return nil
	case prior == "binding":
		return invalidTask("key %q is bound twice (again by %s)", key, b)
	case prior == "reserved":
		return invalidTask("key binding %s uses reserved control key %q", b, key)
	default:
		return invalidTask("key binding %s key %q collides with %s; set keys.%s in task.yaml", b, key, prior, b)
	}
}

// bindingFor matches a key as Bubble Tea reports it, in any accepted spelling.
func (k Keys) bindingFor(written string) (Binding, bool) {
	key, err := ParseKey(written)
	if err != nil {
		return 0, false
	}
	for b, bound := range k {
		if bound == key {
			return Binding(b), true
		}
	}
	return 0, false
}
