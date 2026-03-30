package version

// Version is set at build time for release binaries.
var Version = "dev"

func String() string {
	return Version
}
