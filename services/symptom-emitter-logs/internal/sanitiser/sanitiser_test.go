package sanitiser

import (
	"strings"
	"testing"
)

func TestSanitise_ansi(t *testing.T) {
	raw := "\x1b[31mERROR\x1b[0m: connection refused"
	got := Sanitise(raw)
	if strings.Contains(got, "\x1b") {
		t.Error("ANSI escape sequences not stripped")
	}
	if !strings.Contains(got, "ERROR") {
		t.Error("expected text content preserved")
	}
}

func TestSanitise_angles(t *testing.T) {
	raw := "<script>alert(1)</script>"
	got := Sanitise(raw)
	if strings.Contains(got, "<script>") {
		t.Error("< not escaped")
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("expected &lt;script&gt; in output, got: %q", got)
	}
}

func TestSanitise_lengthCap(t *testing.T) {
	raw := strings.Repeat("x", 2000)
	got := Sanitise(raw)
	// Strip the <evidence>...</evidence> wrapper (2+9+10=21 chars overhead max)
	// The content portion must be <= maxExcerptBytes.
	inner := strings.TrimPrefix(got, "<evidence>\n")
	inner = strings.TrimSuffix(inner, "\n</evidence>")
	if len(inner) > maxExcerptBytes {
		t.Errorf("excerpt not capped: len=%d", len(inner))
	}
}

func TestSanitise_wrapping(t *testing.T) {
	got := Sanitise("hello")
	if !strings.HasPrefix(got, "<evidence>") {
		t.Error("missing <evidence> wrapper prefix")
	}
	if !strings.HasSuffix(got, "</evidence>") {
		t.Error("missing </evidence> wrapper suffix")
	}
}

func TestSanitise_utf8Safe(t *testing.T) {
	// Multi-byte rune near the cap boundary must not be split.
	rune3 := "€" // 3 bytes
	raw := strings.Repeat(rune3, 400) // 1200 bytes
	got := Sanitise(raw)
	inner := strings.TrimPrefix(got, "<evidence>\n")
	inner = strings.TrimSuffix(inner, "\n</evidence>")
	if len(inner) > maxExcerptBytes {
		t.Errorf("excerpt exceeded cap: %d", len(inner))
	}
	// Must be valid UTF-8.
	for _, r := range inner {
		if r == '�' {
			t.Error("replacement rune found: UTF-8 split at rune boundary")
			break
		}
	}
}
