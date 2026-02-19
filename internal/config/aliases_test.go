package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAliasAddAndGet(t *testing.T) {
	saveCfgState(t)
	configPath = t.TempDir() + "/config.yml"
	ResetForTest()

	require.NoError(t, AddAlias("fl", "run list --status=failure"))

	exp, ok := GetAlias("fl")
	assert.True(t, ok)
	assert.Equal(t, "run list --status=failure", exp)
}

func TestAliasAddShellPrefix(t *testing.T) {
	saveCfgState(t)
	configPath = t.TempDir() + "/config.yml"
	ResetForTest()

	require.NoError(t, AddAlias("failing", "!tc run list | jq ."))

	exp, ok := GetAlias("failing")
	assert.True(t, ok)
	assert.Equal(t, "!tc run list | jq .", exp)
	assert.True(t, IsShellAlias("failing"))
	assert.False(t, IsShellAlias("nonexistent"))
}

func TestAliasOverwrite(t *testing.T) {
	saveCfgState(t)
	configPath = t.TempDir() + "/config.yml"
	ResetForTest()

	require.NoError(t, AddAlias("fl", "run list --status=failure"))
	require.NoError(t, AddAlias("fl", "run list --status=success"))

	exp, _ := GetAlias("fl")
	assert.Equal(t, "run list --status=success", exp)
}

func TestAliasDelete(t *testing.T) {
	saveCfgState(t)
	configPath = t.TempDir() + "/config.yml"
	ResetForTest()

	require.NoError(t, AddAlias("fl", "run list"))
	require.NoError(t, DeleteAlias("fl"))

	_, ok := GetAlias("fl")
	assert.False(t, ok)
}

func TestAliasDeleteNonexistent(t *testing.T) {
	saveCfgState(t)
	configPath = t.TempDir() + "/config.yml"
	ResetForTest()

	err := DeleteAlias("nope")
	assert.ErrorContains(t, err, "no such alias")
}

func TestGetAllAliases(t *testing.T) {
	saveCfgState(t)
	configPath = t.TempDir() + "/config.yml"
	ResetForTest()

	require.NoError(t, AddAlias("a", "run list"))
	require.NoError(t, AddAlias("b", "job list"))

	all := GetAllAliases()
	assert.Len(t, all, 2)
	assert.Equal(t, "run list", all["a"])
	assert.Equal(t, "job list", all["b"])
}

func TestGetAllAliasesEmpty(t *testing.T) {
	saveCfgState(t)
	ResetForTest()

	all := GetAllAliases()
	assert.Empty(t, all)
}

func TestAliasPersistence(t *testing.T) {
	saveCfgState(t)
	configPath = t.TempDir() + "/config.yml"
	ResetForTest()

	require.NoError(t, AddAlias("fl", "run list"))

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "aliases")
	assert.Contains(t, string(data), "fl")
}
