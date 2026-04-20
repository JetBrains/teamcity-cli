package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolvePager(T *testing.T) {
	T.Run("returns empty when nothing set", func(t *testing.T) {
		saveCfgState(t)
		t.Setenv(EnvPager, "")
		t.Setenv("PAGER", "")
		cfg = &Config{Servers: map[string]ServerConfig{}}

		assert.Equal(t, "", ResolvePager())
	})

	T.Run("TEAMCITY_PAGER wins over everything", func(t *testing.T) {
		saveCfgState(t)
		t.Setenv(EnvPager, "less -R")
		t.Setenv("PAGER", "more")
		cfg = &Config{Pager: "bat", Servers: map[string]ServerConfig{}}

		assert.Equal(t, "less -R", ResolvePager())
	})

	T.Run("config pager wins over PAGER env", func(t *testing.T) {
		saveCfgState(t)
		t.Setenv(EnvPager, "")
		t.Setenv("PAGER", "more")
		cfg = &Config{Pager: "bat", Servers: map[string]ServerConfig{}}

		assert.Equal(t, "bat", ResolvePager())
	})

	T.Run("PAGER env used when no config value", func(t *testing.T) {
		saveCfgState(t)
		t.Setenv(EnvPager, "")
		t.Setenv("PAGER", "more")
		cfg = &Config{Servers: map[string]ServerConfig{}}

		assert.Equal(t, "more", ResolvePager())
	})
}

func TestSetPager(T *testing.T) {
	saveCfgState(T)
	dir := T.TempDir()
	configPath = filepath.Join(dir, "config.yml")
	cfg = &Config{Servers: map[string]ServerConfig{}}

	require.NoError(T, SetField("pager", "less -R", ""))
	assert.Equal(T, "less -R", cfg.Pager)

	got, err := GetField("pager", "")
	require.NoError(T, err)
	assert.Equal(T, "less -R", got)
}

func TestSetPagerClearsValue(T *testing.T) {
	saveCfgState(T)
	dir := T.TempDir()
	configPath = filepath.Join(dir, "config.yml")
	cfg = &Config{Pager: "less -R", Servers: map[string]ServerConfig{}}

	require.NoError(T, SetField("pager", "", ""))
	assert.Equal(T, "", cfg.Pager)
}

func TestSetPagerRejectsServerFlag(T *testing.T) {
	saveCfgState(T)
	cfg = &Config{Servers: map[string]ServerConfig{}}

	err := SetField("pager", "less", "https://tc.example.com")
	require.Error(T, err)
	assert.Contains(T, err.Error(), "global")
}

func TestGetPagerEmptyWhenUnset(T *testing.T) {
	saveCfgState(T)
	cfg = &Config{Servers: map[string]ServerConfig{}}

	got, err := GetField("pager", "")
	require.NoError(T, err)
	assert.Equal(T, "", got)
}

func TestValidKeysIncludesPager(T *testing.T) {
	assert.Contains(T, ValidKeys(), "pager")
}
