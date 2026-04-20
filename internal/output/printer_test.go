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

func TestPrinterInfof(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Infof("hello %s", "world")
	assert.Equal(t, "hello world", out.String())
}

func TestPrinterPrintField(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.PrintField("ID", "123")
	assert.Equal(t, "ID: 123\n", out.String())
}

func TestPrinterPrintViewHeader(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.PrintViewHeader("My Build", "https://tc.example.com/build/1", func() {
		p.PrintField("Status", "SUCCESS")
	})
	s := out.String()
	assert.Contains(t, s, "My Build")
	assert.Contains(t, s, "Status: SUCCESS")
	assert.Contains(t, s, "https://tc.example.com/build/1")
}

func TestDefaultPrinter(t *testing.T) {
	p := DefaultPrinter()
	assert.NotNil(t, p.Out)
	assert.NotNil(t, p.ErrOut)
	assert.False(t, p.Quiet)
	assert.False(t, p.Verbose)
}

func TestPrinterWarn(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Warn("something %s", "bad")
	assert.Contains(t, out.String(), "something bad")
}

func TestPrinterInfo(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Info("line %d", 42)
	assert.Equal(t, "line 42\n", out.String())
}

func TestPrinterHint(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Hint("Run 'teamcity %s' to continue", "foo")
	assert.Contains(t, out.String(), "Hint: Run 'teamcity foo' to continue")
}

func TestPrinterHintQuiet(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out, Quiet: true}
	p.Hint("should not appear")
	assert.Empty(t, out.String())
}

func TestPrinterEmptyUsesFormatHint(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Empty("No items", "do a thing")
	assert.Contains(t, out.String(), "No items")
	assert.Contains(t, out.String(), "Hint: do a thing")
}
