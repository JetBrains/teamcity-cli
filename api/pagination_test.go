package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteContinuationPathOverridesCountAndFields(t *testing.T) {
	path, err := rewriteContinuationPath(
		"/app/rest/buildTypes?locator=affectedProject:TestProject,count:30,start:30&fields=count,href,nextHref,buildType(id,name)",
		10,
		paginatedFieldsParam("buildType", []string{"id", "name", "projectId"}),
	)
	require.NoError(t, err)

	assert.Contains(t, path, "count%3A10")
	assert.Contains(t, path, "start%3A30")
	assert.Contains(t, path, "projectId")
	assert.Contains(t, path, "nextHref")
}

func TestSetLocatorIntReplacesExistingValue(t *testing.T) {
	locator := setLocatorInt("affectedProject:TestProject,count:30,start:30", "count", 5)
	assert.Equal(t, "affectedProject:TestProject,count:5,start:30", locator)
}

func TestSetLocatorIntAppendsMissingValue(t *testing.T) {
	locator := setLocatorInt("affectedProject:TestProject,start:30", "count", 5)
	assert.Equal(t, "affectedProject:TestProject,start:30,count:5", locator)
}
