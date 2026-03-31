package run_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func TestRunList_invalid_status(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	err := cmdtest.CaptureErr(t, ts.Factory, "run", "list", "--status", "bogus")
	assert.Equal(t, `invalid status "bogus", must be one of: success, failure, running, queued, error, unknown`, err.Error())
}

func TestRunList_invalid_limit(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	err := cmdtest.CaptureErr(t, ts.Factory, "run", "list", "--limit", "0")
	assert.Equal(t, "--limit must be a positive number, got 0", err.Error())
}
