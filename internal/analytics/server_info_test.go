package analytics

import "testing"

func TestServerInfo_RoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	const url = "https://teamcity.example.com"
	v, st := LoadServerInfo(url)
	if v != "" || st != "" {
		t.Errorf("unset cache: got (%q, %q), want empty", v, st)
	}

	if err := SaveServerInfo(url, "2025.11", "cloud"); err != nil {
		t.Fatalf("SaveServerInfo: %v", err)
	}
	v, st = LoadServerInfo(url)
	if v != "2025.11" || st != "cloud" {
		t.Errorf("LoadServerInfo = (%q, %q), want (2025.11, cloud)", v, st)
	}

	// Saving for a different URL must not clobber the existing entry.
	if err := SaveServerInfo("https://other.example.com", "2024.12", "on_prem"); err != nil {
		t.Fatalf("SaveServerInfo other: %v", err)
	}
	v, st = LoadServerInfo(url)
	if v != "2025.11" || st != "cloud" {
		t.Errorf("entry overwritten by sibling: got (%q, %q)", v, st)
	}
}
