package output

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProgramStatusEncoding(t *testing.T) {
	assert.Equal(t, "\x1b]7501;state=idle:app=teamcity\x1b\\", programStatus(ProgramIdle, "", ""))
	message := "run\n\t\x1b\u0085 café"
	encoded := programStatus(ProgramDone, "", message)
	assert.Equal(t, "\x1b]7501;state=done:app=teamcity:msg="+base64.StdEncoding.EncodeToString([]byte("run     café"))+"\x1b\\", encoded)
}

func TestProgramStatusTruncatesUTF8(t *testing.T) {
	sequence := programStatus(ProgramError, "", strings.Repeat("€", 1000))
	encoded := strings.TrimSuffix(strings.SplitN(sequence, ":msg=", 2)[1], "\x1b\\")
	message, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)
	assert.True(t, utf8.Valid(message))
	assert.LessOrEqual(t, len(message), 2048)
	assert.LessOrEqual(t, len(sequence), 4096)
}

func TestProgramStatusSkipsRedirectedOutput(t *testing.T) {
	var out, stderr bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &stderr}
	p.ProgramStatus(ProgramDone, "finished")
	assert.Empty(t, out.String())
	assert.Empty(t, stderr.String())
}
