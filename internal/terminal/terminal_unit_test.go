package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLineEndings(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"unix newlines unchanged", "a\nb\nc", "a\nb\nc"},
		{"windows CRLF to LF", "a\r\nb\r\nc", "a\nb\nc"},
		{"bare CR removed", "a\rb\rc", "abc"},
		{"mixed endings", "a\r\nb\rc\nd", "a\nbc\nd"},
		{"empty string", "", ""},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, normalizeLineEndings(tc.input))
		})
	}
}

func TestExtractExecOutput(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "extracts between markers",
			input: "preamble\n" + execMarker + "\nhello world\n" + execMarker + "\npostamble",
			want:  "hello world",
		},
		{
			name:  "no start marker",
			input: "just some text",
			want:  "",
		},
		{
			name:  "only start marker",
			input: execMarker + "\npartial output",
			want:  "partial output",
		},
		{
			name:  "empty between markers",
			input: execMarker + "\n" + execMarker,
			want:  "",
		},
		{
			name:  "handles CRLF",
			input: "pre\r\n" + execMarker + "\r\nresult\r\n" + execMarker + "\r\npost",
			want:  "result",
		},
		{
			// PSReadLine uses absolute cursor positioning between command output and
			// the closing marker instead of \r\n; stripANSI must remove it.
			name: "powershell absolute cursor positioning between output and closing marker",
			input: "\x1b[m" + execMarker + "\x1b[?25l\r\nhello_5840" +
				"\x1b[11;1H" + execMarker + "\r\n\x1b[?25h",
			want: "hello_5840",
		},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, extractExecOutput(tc.input))
		})
	}
}

func TestStripANSI(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"CSI color sequence", "\x1b[93mwarn\x1b[m", "warn"},
		{"CSI cursor hide/show", "\x1b[?25lA\x1b[?25h", "A"},
		{"CSI absolute cursor positioning", "A\x1b[11;1HB", "AB"},
		{"OSC window title with BEL terminator", "\x1b]0;title\aX", "X"},
		{"OSC with ESC \\ terminator", "\x1b]0;title\x1b\\X", "X"},
		{"preserves CR and LF", "line1\r\nline2\n", "line1\r\nline2\n"},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, stripANSI(tc.input))
		})
	}
}

func TestNormalizeEnterKeystroke(T *testing.T) {
	T.Parallel()

	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{"solitary LF becomes CR (broken-console Enter)", []byte("\n"), []byte("\r")},
		{"solitary CR unchanged (conformant Enter)", []byte("\r"), []byte("\r")},
		{"solitary printable char unchanged", []byte("a"), []byte("a")},
		{"solitary Ctrl+C unchanged", []byte{0x03}, []byte{0x03}},
		{"CRLF pair not rewritten (paste or combined read)", []byte("\r\n"), []byte("\r\n")},
		{"LF inside multi-byte paste left as LF", []byte("line1\nline2"), []byte("line1\nline2")},
		{"escape sequence for arrow unchanged", []byte{0x1b, '[', 'A'}, []byte{0x1b, '[', 'A'}},
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			buf := append([]byte(nil), tc.in...)
			normalizeEnterKeystroke(buf)
			assert.Equal(t, tc.want, buf)
		})
	}
}
