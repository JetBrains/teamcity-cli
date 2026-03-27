package output

import (
	"fmt"
	"testing"
)

func TestOutputFunctions(T *testing.T) {
	// Cannot use T.Parallel() because this test modifies package-level Quiet/Verbose
	oldQuiet := Quiet
	oldVerbose := Verbose
	T.Cleanup(func() {
		Quiet = oldQuiet
		Verbose = oldVerbose
	})

	for _, quiet := range []bool{true, false} {
		T.Run(fmt.Sprintf("quiet=%v", quiet), func(t *testing.T) {
			Quiet = quiet
			Success("test %s", "message")
			Info("test %s", "info")
			Infof("test %s", "infof")
			Warn("test %s", "warn")
		})
	}

	for _, verbose := range []bool{true, false} {
		T.Run(fmt.Sprintf("verbose=%v", verbose), func(t *testing.T) {
			Verbose = verbose
			Debug("test %s", "debug")
		})
	}

	T.Run("PrintField", func(t *testing.T) {
		PrintField("Name", "value")
	})

	T.Run("PrintViewHeader", func(t *testing.T) {
		PrintViewHeader("Title", "https://example.com", func() {
			PrintField("Status", "OK")
		})
	})
}
