package status

import "testing"

func TestFormatBuildStatus(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"SUCCESS", "success"},
		{"  FAILURE  ", "failure"},
		{"Running", "running"},
	}
	for _, tc := range tests {
		if got := FormatBuildStatus(tc.in); got != tc.want {
			t.Errorf("FormatBuildStatus(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
