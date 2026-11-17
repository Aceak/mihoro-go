package mihoro

import (
	"bufio"
	"strings"
	"testing"
)

func TestPrompterConsumesLinesInOrder(t *testing.T) {
	p := &prompter{in: bufio.NewReader(strings.NewReader("y\nn\n\nmy-secret\n"))}

	// Each answer must be consumed in order, including empty lines; this is
	// the piped multi-line input case that a per-prompt Scanner used to break.
	if !p.Bool("first", "N") {
		t.Error("first answer should be true (y)")
	}
	if p.Bool("second", "Y") {
		t.Error("second answer should be false (n)")
	}
	if got, ok := p.Line("third"); !ok || got != "" {
		t.Errorf("third answer should be an empty line, got %q ok=%v", got, ok)
	}
	if got, ok := p.Line("fourth"); !ok || got != "my-secret" {
		t.Errorf("fourth answer = %q, want my-secret", got)
	}
}

func TestPrompterEOF(t *testing.T) {
	p := &prompter{in: bufio.NewReader(strings.NewReader(""))}

	if !p.Bool("q", "Y") {
		t.Error("EOF Bool should return default Y")
	}
	if p.Bool("q", "N") {
		t.Error("EOF Bool should return default N")
	}
	if got, ok := p.Line("q"); ok || got != "" {
		t.Errorf("EOF Line should return ('', false), got (%q, %v)", got, ok)
	}
	if got := p.String("q", "fallback"); got != "fallback" {
		t.Errorf("EOF String should return default, got %q", got)
	}
}

func TestPrompterLastLineWithoutNewline(t *testing.T) {
	p := &prompter{in: bufio.NewReader(strings.NewReader("yes"))}
	if !p.Bool("q", "N") {
		t.Error("last line without trailing newline should still be read")
	}
}

func TestPrompterLineKeepsSpaces(t *testing.T) {
	// Password-like input must not be silently modified: only the trailing
	// newline is removed, spaces are preserved for the caller to validate.
	p := &prompter{in: bufio.NewReader(strings.NewReader("  padded value  \n"))}
	got, ok := p.Line("q")
	if !ok || got != "  padded value  " {
		t.Errorf("Line() = (%q, %v), want the raw line with spaces preserved", got, ok)
	}
}
