package sanitiser

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

const maxExcerptBytes = 1024

// ansiEscape matches ANSI/VT100 escape sequences.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// Sanitise strips ANSI codes, replaces < > with safe variants, caps the
// excerpt at maxExcerptBytes, and wraps it in an <evidence> fenced block.
func Sanitise(raw string) string {
	s := ansiEscape.ReplaceAllString(raw, "")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")

	// Trim to byte budget (UTF-8-safe).
	if len(s) > maxExcerptBytes {
		s = truncateUTF8(s, maxExcerptBytes)
	}

	return "<evidence>\n" + s + "\n</evidence>"
}

func truncateUTF8(s string, maxBytes int) string {
	b := []byte(s)
	if len(b) <= maxBytes {
		return s
	}
	b = b[:maxBytes]
	// Walk backwards until we land on a valid rune boundary.
	for len(b) > 0 && !utf8.RuneStart(b[len(b)-1]) {
		b = b[:len(b)-1]
	}
	return string(b)
}
