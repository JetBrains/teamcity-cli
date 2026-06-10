package migrate

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/analytics"
	"github.com/JetBrains/teamcity-cli/internal/migrate"
	"github.com/stretchr/testify/assert"
)

func TestMigrateSourceField(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []migrate.CIConfig
		want string
	}{
		{"empty", nil, analytics.MigrateSourceNone},
		{"single gha", []migrate.CIConfig{{Source: migrate.GitHubActions}}, analytics.MigrateSourceGitHubActions},
		{"single bamboo", []migrate.CIConfig{{Source: migrate.Bamboo}}, analytics.MigrateSourceBamboo},
		{"multi same", []migrate.CIConfig{{Source: migrate.GitHubActions}, {Source: migrate.GitHubActions}}, analytics.MigrateSourceGitHubActions},
		{"mixed", []migrate.CIConfig{{Source: migrate.GitHubActions}, {Source: migrate.Bamboo}}, analytics.MigrateSourceMixed},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, migrateSourceField(c.in))
		})
	}
}

func TestMigrateOutcomeField(t *testing.T) {
	t.Parallel()
	clean := &migrate.ConversionResult{}
	partial := &migrate.ConversionResult{ManualSetup: []string{"do thing"}}
	cfg := []migrate.CIConfig{{Source: migrate.GitHubActions}}

	assert.Equal(t, analytics.MigrateOutcomeNothingFound, migrateOutcomeField(nil, nil))
	assert.Equal(t, analytics.MigrateOutcomeFailed, migrateOutcomeField(cfg, nil))
	assert.Equal(t, analytics.MigrateOutcomePartial, migrateOutcomeField(cfg, []*migrate.ConversionResult{partial}))
	assert.Equal(t, analytics.MigrateOutcomeClean, migrateOutcomeField(cfg, []*migrate.ConversionResult{clean}))

	// A config that failed to convert (fewer results than configs) makes the outcome partial.
	cfgTwo := []migrate.CIConfig{{Source: migrate.GitHubActions}, {Source: migrate.Bamboo}}
	assert.Equal(t, analytics.MigrateOutcomePartial, migrateOutcomeField(cfgTwo, []*migrate.ConversionResult{clean}))
}

func TestMigrateValidationField(t *testing.T) {
	t.Parallel()
	good := []*migrate.ConversionResult{{}}
	bad := []*migrate.ConversionResult{{ValidationError: "schema mismatch"}}

	assert.Equal(t, analytics.MigrateValidationSkipped, migrateValidationField(&migrateOptions{noValidate: true}, good))
	assert.Equal(t, analytics.MigrateValidationInvalid, migrateValidationField(&migrateOptions{}, bad))
	assert.Equal(t, analytics.MigrateValidationValid, migrateValidationField(&migrateOptions{}, good))
}
