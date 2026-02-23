//go:build integration

package api_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerforceUploadDiffChanges(T *testing.T) {
	T.Parallel()

	patch := []byte(`--- a/depot/main/test.txt
+++ b/depot/main/test.txt
@@ -1 +1 @@
-Hello from Perforce
+Hello from Perforce - modified in personal build
`)

	changeID, err := client.UploadDiffChanges(patch, "Perforce personal build test")
	require.NoError(T, err)
	assert.NotEmpty(T, changeID)
	T.Logf("Uploaded Perforce diff as change ID: %s", changeID)
}
