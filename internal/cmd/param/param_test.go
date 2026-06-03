package param_test

import (
	"strings"
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

func TestParamListProjectWeb(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "project", "param", "list", "TestProject", "--web")
	want := ts.URL + "/admin/editProject.html?projectId=TestProject&tab=projectParams"
	if !strings.Contains(out, want) {
		t.Fatalf("--web output = %q, want it to contain %q", out, want)
	}
}

func TestParamListJobWeb(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "param", "list", "TestProject_Build", "--web")
	want := ts.URL + "/admin/editBuildParams.html?id=buildType:TestProject_Build"
	if !strings.Contains(out, want) {
		t.Fatalf("--web output = %q, want it to contain %q", out, want)
	}
}

// --web must validate the owner exists before emitting its params URL (not exit 0 on a bad id).
func TestParamListWebValidatesOwner(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "", "project", "param", "list", "NonExistentProject123456", "--web")
}

func TestParamRequiresSubcommand(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "requires a subcommand", "project", "param")
	cmdtest.RunCmdWithFactoryExpectErr(t, f, "requires a subcommand", "job", "param")
}

func TestParamListWithoutIDOrLink(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "project id is required", "project", "param", "list")
}

func TestParamGetMissingArgs(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "accepts between 1 and 2 arg(s)", "project", "param", "get")
}

func TestParamSetMissingArgs(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)
	f := ts.Factory

	cmdtest.RunCmdWithFactoryExpectErr(t, f, "accepts between 2 and 3 arg(s)", "project", "param", "set", "name")
}
