package setting_test

import (
	"strings"
	"testing"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
)

func TestSettingsList(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "settings", "list", "TestProject_Build")
	if !strings.Contains(out, "buildNumberPattern") {
		t.Fatalf("list output = %q, want it to contain buildNumberPattern", out)
	}
}

func TestSettingsListEmpty(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "settings", "list", "EmptySettingsJob")
	if !strings.Contains(out, "No settings found") {
		t.Fatalf("empty list output = %q, want it to contain 'No settings found'", out)
	}
}

func TestSettingsListJSON(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "settings", "list", "TestProject_Build", "--json")
	if !strings.Contains(out, "\"buildNumberPattern\"") {
		t.Fatalf("--json output = %q, want it to contain buildNumberPattern", out)
	}
}

func TestSettingsGet(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "settings", "get", "TestProject_Build", "executionTimeoutMin")
	if !strings.Contains(out, "10") {
		t.Fatalf("get output = %q, want it to contain 10", out)
	}
}

func TestSettingsSet(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "settings", "set", "TestProject_Build", "buildNumberPattern", "2.0.%build.counter%")
	if !strings.Contains(out, "buildNumberPattern") {
		t.Fatalf("set output = %q, want it to confirm buildNumberPattern", out)
	}
}

// --web must fetch (validating the id) before emitting the settings URL.
func TestSettingsListWeb(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	out := cmdtest.CaptureOutput(t, ts.Factory, "job", "settings", "list", "TestProject_Build", "--web")
	want := ts.URL + "/admin/editBuildTypeGeneralSettings.html?id=buildType:TestProject_Build"
	if !strings.Contains(out, want) {
		t.Fatalf("--web output = %q, want it to contain %q", out, want)
	}
}

func TestSettingsRequiresID(t *testing.T) {
	ts := cmdtest.SetupMockClient(t)

	cmdtest.RunCmdWithFactoryExpectErr(t, ts.Factory, "job id is required", "job", "settings", "list")
}
