package output

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

// withASCII flips the global ASCII mode for the duration of a (non-parallel) test.
func withASCII(t *testing.T, v bool) {
	t.Helper()
	prev := ASCII
	ASCII = v
	t.Cleanup(func() { ASCII = prev })
}

func TestSymSelectsMode(t *testing.T) {
	withASCII(t, false)
	assert.Equal(t, "→", Sym().Arrow)
	withASCII(t, true)
	assert.Equal(t, "->", Sym().Arrow)
}

func TestASCIISymbolsArePureASCII(t *testing.T) {
	t.Parallel()
	v := reflect.ValueOf(asciiSymbols)
	for i := range v.NumField() {
		field := v.Type().Field(i).Name
		for _, r := range v.Field(i).String() {
			assert.Lessf(t, r, rune(0x80), "asciiSymbols.%s contains non-ASCII %q", field, r)
		}
	}
}

func TestStatusIconASCIIMatchesPlain(t *testing.T) {
	withASCII(t, true)
	cases := []struct{ status, state, text string }{
		{"SUCCESS", "", ""},
		{"FAILURE", "", ""},
		{"UNKNOWN", "", "Canceled (user)"},
		{"OTHER", "", ""},
		{"", "running", ""},
		{"", "queued", ""},
	}
	for _, c := range cases {
		got := ansi.Strip(StatusIcon(c.status, c.state, c.text))
		assert.Equal(t, PlainStatusIcon(c.status, c.state, c.text), got)
	}
}

func TestPrintTreeASCIIConnectors(t *testing.T) {
	withASCII(t, true)
	var buf bytes.Buffer
	p := &Printer{Out: &buf}
	p.PrintTree(TreeNode{Label: "root", Children: []TreeNode{
		{Label: "a", Children: []TreeNode{{Label: "a1"}}},
		{Label: "b"},
	}})
	out := buf.String()
	assert.Contains(t, out, "|-- a")
	assert.Contains(t, out, "`-- b")
	for _, r := range out {
		assert.Less(t, r, rune(0x80), "tree output must be ASCII-only")
	}
}
