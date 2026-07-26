package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestHighlight_ColoursCodeItRecognises(t *testing.T) {
	line := `	name := "hello" // a greeting`
	got := Highlight("main.go", line)

	if got == line {
		t.Fatal("a Go line should come back coloured")
	}
	if ansi.Strip(got) != line {
		t.Errorf("stripped = %q, want the line unchanged underneath the colour: %q",
			ansi.Strip(got), line)
	}
}

func TestHighlight_KeepsLeadingIndentation(t *testing.T) {
	// Losing the indentation would flatten every block in the diff.
	line := "        return None"
	got := ansi.Strip(Highlight("thing.py", line))
	if !strings.HasPrefix(got, "        ") {
		t.Errorf("got %q, want the eight spaces kept", got)
	}
}

func TestHighlight_LeavesWhatItCannotRead(t *testing.T) {
	line := "some plain text"
	if got := Highlight("mystery.zzz", line); got != line {
		t.Errorf("got %q, want an unknown language returned untouched", got)
	}
	if got := Highlight("main.go", "   "); got != "   " {
		t.Errorf("got %q, want a blank line returned untouched", got)
	}
}

func TestHighlight_SameLexerServesTheWholeFile(t *testing.T) {
	// The cache is what makes highlighting a thousand-line diff affordable.
	first := Highlight("cached.go", "func main() {}")
	second := Highlight("cached.go", "func main() {}")
	if first != second {
		t.Error("the same line in the same file should render identically")
	}
	if _, ok := lexerCache.Load("cached.go"); !ok {
		t.Error("the lexer for a path should be remembered")
	}
}
