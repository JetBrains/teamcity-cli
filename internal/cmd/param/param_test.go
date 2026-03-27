package param_test

import (
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func TestParamListProject(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(t, f, "project", "param", "list", "TestProject")
	cmdtest.RunCmdWithFactory(t, f, "project", "param", "list", "TestProject", "--json")
}

func TestParamGetProject(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(t, f, "project", "param", "get", "TestProject", "param1")
}

func TestParamSetProject(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(t, f, "project", "param", "set", "TestProject", "myParam", "myValue")
	cmdtest.RunCmdWithFactory(t, f, "project", "param", "set", "TestProject", "secret", "s3cret", "--secure")
}

func TestParamDeleteProject(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(t, f, "project", "param", "delete", "TestProject", "myParam")
}

func TestParamListJob(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(t, f, "job", "param", "list", "TestProject_Build")
	cmdtest.RunCmdWithFactory(t, f, "job", "param", "list", "TestProject_Build", "--json")
}

func TestParamGetJob(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(t, f, "job", "param", "get", "TestProject_Build", "param1")
}

func TestParamSetJob(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(t, f, "job", "param", "set", "TestProject_Build", "myParam", "myValue")
}

func TestParamDeleteJob(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactory(t, f, "job", "param", "delete", "TestProject_Build", "myParam")
}

func TestParamRequiresSubcommand(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "requires a subcommand", "project", "param")
	cmdtest.RunCmdWithFactoryExpectErr(t, f, "requires a subcommand", "job", "param")
}

func TestParamListMissingArgs(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "accepts 1 arg(s)", "project", "param", "list")
}

func TestParamGetMissingArgs(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "accepts 2 arg(s)", "project", "param", "get", "TestProject")
}

func TestParamSetMissingArgs(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "accepts 3 arg(s)", "project", "param", "set", "TestProject", "name")
}
