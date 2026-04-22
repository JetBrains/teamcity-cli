package output

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/JetBrains/teamcity-cli/api"
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

func TestPrinterProgress(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Progress("hello %s", "world")
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

func TestPrinterTip(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Tip("Run 'teamcity %s' to continue", "foo")
	assert.Contains(t, out.String(), "Tip: Run 'teamcity foo' to continue")
}

func TestPrinterTipQuiet(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out, Quiet: true}
	p.Tip("should not appear")
	assert.Empty(t, out.String())
}

func TestPrinterEmptyUsesFormatTip(t *testing.T) {
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}
	p.Empty("No items", "do a thing")
	assert.Contains(t, out.String(), "No items")
	assert.Contains(t, out.String(), "Tip: do a thing")
}

// TestTipFor_PermissionOnlyWhenIdentified pins that CatPermission picks the identified-permission branch when parseable, and the generic re-auth tip otherwise.
func TestTipFor_PermissionOnlyWhenIdentified(t *testing.T) {
	parsed := newForbidden(`{"errors":[{"message":"You do not have \"Comment build\" permission in project with internal id: 'p1'"}]}`)
	assert.Contains(t, tipFor(parsed), "COMMENT_BUILD")

	ambiguous := newForbidden(`{"errors":[{"message":"Build was not canceled. Probably not sufficient permissions."}]}`)
	assert.Equal(t, "Re-authenticate with broader permissions via 'teamcity auth login'", tipFor(ambiguous))
}

// TestTipFor_Permission_AuthSourceBranches covers each AuthSource branch in permissionTip.
func TestTipFor_Permission_AuthSourceBranches(t *testing.T) {
	t.Parallel()

	const (
		serverDesc = "Comment build"
		wantEnum   = "COMMENT_BUILD"
	)
	body := `{"errors":[{"message":"You do not have \"` + serverDesc + `\" permission in project with internal id: 'p1'"}]}`

	tests := []struct {
		name   string
		src    api.AuthSource
		expect string
	}{
		{"pkce", api.AuthSourcePKCE, "permissions picker"},
		{"env", api.AuthSourceEnv, "TEAMCITY_TOKEN"},
		{"build", api.AuthSourceBuild, "Build-level credentials"},
		{"guest", api.AuthSourceGuest, "Guest access lacks"},
		{"manual", api.AuthSourceManual, "Generate a new access token"},
		{"unknown", api.AuthSourceUnknown, "Generate a new access token"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := newForbidden(body)
			pe, ok := errors.AsType[*api.PermissionError](err)
			require.True(t, ok)
			pe.AuthSource = tc.src
			got := tipFor(pe)
			assert.Contains(t, got, tc.expect)
			assert.Contains(t, got, wantEnum)
		})
	}
}

// TestTipFor_Permission_UnknownDescriptionFallsBackToQuoted pins that unmapped server descriptions appear quoted verbatim in the tip.
func TestTipFor_Permission_UnknownDescriptionFallsBackToQuoted(t *testing.T) {
	t.Parallel()
	err := newForbidden(`{"errors":[{"message":"You do not have \"Some unknown perm\" permission in project with internal id: 'p1'"}]}`)
	pe, ok := errors.AsType[*api.PermissionError](err)
	require.True(t, ok)
	pe.AuthSource = api.AuthSourcePKCE
	assert.Contains(t, tipFor(pe), `"Some unknown perm"`)
}

// TestPrinterConcurrentWrites regresses F17: without the mutex, -race fires and writes interleave.
func TestPrinterConcurrentWrites(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	p := &Printer{Out: &out, ErrOut: &out}

	const n = 50
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			switch i % 3 {
			case 0:
				p.Progress("progress-%d\n", i)
			case 1:
				p.Success("success-%d", i)
			case 2:
				p.Warn("warn-%d", i)
			}
		})
	}
	wg.Wait()

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	require.Len(t, lines, n, "expected one line per goroutine (buffer corruption implies torn writes)")
	for i := range n {
		var prefix string
		switch i % 3 {
		case 0:
			prefix = "progress-"
		case 1:
			prefix = "success-"
		case 2:
			prefix = "warn-"
		}
		tag := fmt.Sprintf("%s%d", prefix, i)
		assert.True(t, slices.ContainsFunc(lines, func(l string) bool { return strings.Contains(l, tag) }),
			"tag %s missing — write was torn or reordered", tag)
	}
}

func newForbidden(body string) error {
	return api.ErrorFromResponse(&http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(body)),
	})
}
