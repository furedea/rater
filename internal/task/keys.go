package task

import (
	"fmt"
	"strings"
)

// Fixed control keys of the annotate screen. Bindings may not reuse them.
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

// Keys maps each binding to the key, in Bubble Tea's key notation, that triggers it.
type Keys [bindingCount]string

var defaultKeys = Keys{
	BindingScrollUp:   "up",
	BindingScrollDown: "down",
	BindingPageUp:     "pgup",
	BindingPageDown:   "pgdown",
	BindingNextHunk:   "]",
	BindingPrevHunk:   "[",
}

var controlKeys = []string{KeyQuit, KeyInterrupt, KeyHold, KeyPrev, KeyNext, KeyUndo, KeyNote}

// newKeys applies overrides to the defaults and rejects keys that another
// binding, a control key, or a label already uses.
func newKeys(overrides map[Binding]string, labels []Label) (Keys, error) {
	keys := defaultKeys
	for b, key := range overrides {
		keys[b] = key
	}
	owner := map[string]string{}
	for _, k := range controlKeys {
		owner[k] = "reserved"
	}
	for _, l := range labels {
		owner[string(l.Key)] = fmt.Sprintf("label %q", l.Name)
	}
	for _, b := range Bindings() {
		if err := claimKey(owner, b, keys[b]); err != nil {
			return Keys{}, err
		}
	}
	return keys, nil
}

func claimKey(owner map[string]string, b Binding, key string) error {
	if strings.TrimSpace(key) != key || key == "" {
		return invalidTask("key binding %s has unusable key %q", b, key)
	}
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

func (k Keys) bindingFor(key string) (Binding, bool) {
	for b, bound := range k {
		if bound == key {
			return Binding(b), true
		}
	}
	return 0, false
}
