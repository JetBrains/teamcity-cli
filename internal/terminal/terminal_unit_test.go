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
	}

	for _, tc := range tests {
		T.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, extractExecOutput(tc.input))
		})
	}
}
