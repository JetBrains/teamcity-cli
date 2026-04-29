package project

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	openBrowser = func(string) error { return nil }
	os.Exit(m.Run())
}
