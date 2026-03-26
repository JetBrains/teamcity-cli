package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrinterSuccess(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Success("done %d", 1)
	assert.Contains(t, out.String(), "done 1")
}

func TestPrinterQuiet(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out, Quiet: true}
	p.Success("hidden")
	p.Info("hidden")
	p.Warn("hidden")
	assert.Empty(t, out.String())
}

func TestPrinterDebugVerbose(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out, Verbose: true}
	p.Debug("trace %s", "info")
	assert.Contains(t, out.String(), "trace info")
}

func TestPrinterDebugSilent(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Debug("hidden")
	assert.Empty(t, out.String())
}

func TestPrinterJSON(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	err := p.PrintJSON(map[string]int{"count": 5})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `"count": 5`)
}
