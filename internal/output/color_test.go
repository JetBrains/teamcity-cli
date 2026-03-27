package output

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func TestPrinterMethods(T *testing.T) {
	for _, quiet := range []bool{true, false} {
		T.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			var out, errOut bytes.Buffer
			p := &Printer{Out: &out, ErrOut: &errOut, Quiet: quiet}
			p.Success("test %s", "message")
			p.Info("test %s", "info")
			p.Infof("test %s", "infof")
			p.Warn("test %s", "warn")
			if quiet {
				if out.Len() != 0 {
					t.Errorf("expected no output when quiet, got %q", out.String())
				}
			} else {
				if !strings.Contains(out.String(), "test message") {
					t.Errorf("expected success output, got %q", out.String())
				}
			}
		})
	}

	for _, verbose := range []bool{true, false} {
		T.Run(fmt.Sprintf("verbose=%v", verbose), func(t *testing.T) {
			var out, errOut bytes.Buffer
			p := &Printer{Out: &out, ErrOut: &errOut, Verbose: verbose}
			p.Debug("test %s", "debug")
			if verbose && errOut.Len() == 0 {
				t.Error("expected debug output when verbose")
			}
			if !verbose && errOut.Len() != 0 {
				t.Error("expected no debug output when not verbose")
			}
		})
	}

	T.Run("PrintField", func(t *testing.T) {
		var out bytes.Buffer
		p := &Printer{Out: &out}
		p.PrintField("Name", "value")
		if !strings.Contains(out.String(), "Name: value") {
			t.Errorf("expected field output, got %q", out.String())
		}
	})

	T.Run("PrintViewHeader", func(t *testing.T) {
		var out bytes.Buffer
		p := &Printer{Out: &out}
		p.PrintViewHeader("Title", "https://example.com", func() {
			p.PrintField("Status", "OK")
		})
		if !strings.Contains(out.String(), "example.com") {
			t.Errorf("expected header output, got %q", out.String())
		}
	})
}
