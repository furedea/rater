package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

	"github.com/furedea/rater/internal/task"
)

// codeStyle is the Chroma palette. Output is always 24-bit colour; Bubble Tea
// downsamples it to the terminal's profile when it writes the frame.
const codeStyle = "monokai"

// HasLanguage reports whether the highlighter knows name, given as a language
// name, alias, or file name.
func HasLanguage(name string) bool { return lexers.Get(name) != nil }

// languageOf picks the item's language when it names a known one, else the task's.
func languageOf(f task.Field, item task.Item) string {
	if name, ok := item.Values[f.LanguageField]; ok && f.LanguageField != "" && HasLanguage(name) {
		return name
	}
	return f.Language
}

// writeCode highlights src as language, guessing the language when it is empty.
func writeCode(w io.Writer, src, language string) error {
	src = strings.TrimRight(src, "\n")
	tokens, err := lexerFor(src, language).Tokenise(nil, src)
	if err != nil {
		return fmt.Errorf("tokenise %s: %w", language, err)
	}
	if err := formatters.TTY16m.Format(w, styles.Get(codeStyle), tokens); err != nil {
		return fmt.Errorf("format %s: %w", language, err)
	}
	return nil
}

func lexerFor(src, language string) chroma.Lexer {
	l := lexers.Get(language)
	if l == nil {
		l = lexers.Analyse(src)
	}
	if l == nil {
		l = lexers.Fallback
	}
	return chroma.Coalesce(l)
}
