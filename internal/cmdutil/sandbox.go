package cmdutil

import (
	"os"

	"github.com/JetBrains/teamcity-cli/internal/output"
)

// DetectSandbox names the agent sandbox this process runs in, or returns "" outside one.
// It reads only variables set inside the sandbox; commands an agent runs outside it don't get them.
func DetectSandbox() string {
	switch {
	case os.Getenv("CODEX_SANDBOX") != "": // macOS only
		return "the Codex sandbox"
	case os.Getenv("SANDBOX_RUNTIME") != "": // sandbox-runtime, used by Claude Code and Junie
		return "an agent sandbox"
	}
	return ""
}

// CredentialsTip returns the tip for a token that can't be read, or "" when the generic authentication hints apply.
func CredentialsTip(serverURL string, keyringLogin bool) string {
	if sandbox := DetectSandbox(); sandbox != "" {
		return output.TipSandboxed(sandbox)
	}
	// Codex sets this on every OS, also for commands it runs outside its sandbox, so it only hints at one.
	if keyringLogin || os.Getenv("CODEX_SANDBOX_NETWORK_DISABLED") != "" {
		return output.TipKeyringHiddenFor(serverURL)
	}
	return ""
}
