package analytics

import "testing"

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name        string
		envDoNot    string
		envTC       string
		configEnabl bool
		wantEnabled bool
		wantReason  OptOutReason
	}{
		{"all defaults on", "", "", true, true, OptOutNone},
		{"do not track wins", "1", "", true, false, OptOutDoNotTrack},
		{"do not track lowercased", "true", "", true, false, OptOutDoNotTrack},
		{"do not track ignored when zero", "0", "", true, true, OptOutNone},
		{"env disable wins over config on", "", "0", true, false, OptOutEnv},
		{"env enable=1 keeps default on", "", "1", true, true, OptOutNone},
		{"config disable", "", "", false, false, OptOutConfig},
		{"do not track beats env=1 and config", "1", "1", true, false, OptOutDoNotTrack},
		{"env=false beats config on", "", "false", true, false, OptOutEnv},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvDoNotTrack, tc.envDoNot)
			t.Setenv(EnvAnalytics, tc.envTC)
			gotEnabled, gotReason := IsEnabled(tc.configEnabl)
			if gotEnabled != tc.wantEnabled {
				t.Errorf("IsEnabled enabled = %v, want %v", gotEnabled, tc.wantEnabled)
			}
			if gotReason != tc.wantReason {
				t.Errorf("IsEnabled reason = %q, want %q", gotReason, tc.wantReason)
			}
		})
	}
}

func TestIsTruthy(t *testing.T) {
	for _, on := range []string{"1", "true", "TRUE", "yes", "Yes", "on", " on "} {
		if !isTruthy(on) {
			t.Errorf("isTruthy(%q) = false, want true", on)
		}
	}
	for _, off := range []string{"", "0", "false", "no", "off", "anything"} {
		if isTruthy(off) {
			t.Errorf("isTruthy(%q) = true, want false", off)
		}
	}
}
