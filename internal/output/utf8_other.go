//go:build !windows

package output

// ConsoleSupportsUTF8 reports whether the active console can render UTF-8.
// Non-Windows terminals are assumed UTF-8 capable; force ASCII with
// TEAMCITY_ASCII when that assumption is wrong.
func ConsoleSupportsUTF8() bool { return true }
