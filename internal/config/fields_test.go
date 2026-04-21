package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolvePager(T *testing.T) {
	T.Run("returns empty when nothing set", func(t *testing.T) {
		saveCfgState(t)
		t.Setenv(EnvPager, "")
		t.Setenv("PAGER", "")

		assert.Equal(t, "", ResolvePager())
	})

	T.Run("TEAMCITY_PAGER wins over PAGER", func(t *testing.T) {
		saveCfgState(t)
		t.Setenv(EnvPager, "less -R")
		t.Setenv("PAGER", "more")

		assert.Equal(t, "less -R", ResolvePager())
	})

	T.Run("PAGER used when TEAMCITY_PAGER unset", func(t *testing.T) {
		saveCfgState(t)
		t.Setenv(EnvPager, "")
		t.Setenv("PAGER", "more")

		assert.Equal(t, "more", ResolvePager())
	})
}
