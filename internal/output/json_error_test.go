package output_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintJSONError(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	output.PrintJSONError(&buf, output.ErrCodeAuth, "Authentication failed", "teamcity auth login")

	var got output.JSONError
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, output.ErrCodeAuth, got.Error.Code)
	assert.Equal(t, "Authentication failed", got.Error.Message)
	assert.Equal(t, "teamcity auth login", got.Error.Suggestion)
}

func TestPrintJSONError_noSuggestion(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	output.PrintJSONError(&buf, output.ErrCodeInternal, "something broke", "")

	var got output.JSONError
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))
	assert.Equal(t, output.ErrCodeInternal, got.Error.Code)
	assert.Empty(t, got.Error.Suggestion)
}
