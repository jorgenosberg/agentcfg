// Package paths resolves the base directory agentcfg derives all of its
// paths from. AGENTCFG_HOME overrides it for isolated testing; otherwise
// the real user home is used.
package paths

import "os"

// Home returns the base directory for agentcfg path resolution.
// When AGENTCFG_HOME is set it is returned verbatim, redirecting both
// agentcfg's own state (~/.agentcfg/*) and the agent catalog
// (~/.claude, ~/.codex, ...) under the sandbox. Otherwise os.UserHomeDir().
func Home() (string, error) {
	if h := os.Getenv("AGENTCFG_HOME"); h != "" {
		return h, nil
	}
	return os.UserHomeDir()
}
