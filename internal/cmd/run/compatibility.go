package run

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

// agentListInlineLimit is the max agents listed before we collapse to a pool summary.
const agentListInlineLimit = 20

// reasonProbeLimit caps how many incompatibleBuildTypes entries to scan per agent when fetching reasons.
const reasonProbeLimit = 20000

// reasonProbeAgents caps how many incompatible agents we probe for reasons to avoid long waits.
const reasonProbeAgents = 5

// compatibilityWaitKeywords identifies wait reasons that hint at agent compatibility issues.
var compatibilityWaitKeywords = []string{
	"compatible agents",
	"outdated, waiting for upgrade",
}

// waitReasonIsCompatibility returns true when the wait reason suggests an agent compatibility problem.
func waitReasonIsCompatibility(waitReason string) bool {
	lower := strings.ToLower(waitReason)
	return slices.ContainsFunc(compatibilityWaitKeywords, func(kw string) bool {
		return strings.Contains(lower, kw)
	})
}

// renderBuildCompatibility prints compatible/incompatible agents for a queued build; errors are best-effort.
func renderBuildCompatibility(w io.Writer, client api.ClientInterface, build *api.Build) {
	if build.State != "queued" {
		return
	}

	compat, errC := client.GetBuildCompatibleAgents(build.ID)
	incompat, errI := client.GetBuildIncompatibleAgents(build.ID)

	if errC != nil && errI != nil {
		_, _ = fmt.Fprintf(w, "\n%s %v\n", output.Faint("Could not load agent compatibility:"), errC)
		return
	}

	_, _ = fmt.Fprintln(w)

	if compat != nil {
		renderAgentGroup(w, "Compatible agents", compat.Count, compat.Agents, output.Green)
	} else if errC != nil {
		_, _ = fmt.Fprintf(w, "%s %v\n", output.Faint("Compatible agents unavailable:"), errC)
	}

	if incompat != nil {
		renderAgentGroup(w, "Incompatible agents", incompat.Count, incompat.Agents, output.Yellow)
		renderIncompatibilityReasons(w, client, build.BuildTypeID, incompat.Agents)
	} else if errI != nil {
		_, _ = fmt.Fprintf(w, "%s %v\n", output.Faint("Incompatible agents unavailable:"), errI)
	}
}

// renderAgentGroup prints a header plus either the agent list or a pool-count summary.
func renderAgentGroup(w io.Writer, title string, total int, agents []api.Agent, colorize func(a ...any) string) {
	_, _ = fmt.Fprintf(w, "%s (%d)", colorize(title), total)
	if total == 0 {
		_, _ = fmt.Fprintln(w)
		return
	}
	_, _ = fmt.Fprintln(w)

	if len(agents) <= agentListInlineLimit {
		pools := groupAgentsByPool(agents)
		for _, pool := range pools {
			_, _ = fmt.Fprintf(w, "  %s\n", output.Faint("["+pool.name+"]"))
			for _, a := range pool.agents {
				_, _ = fmt.Fprintf(w, "    %s%s\n", a.Name, agentStatusSuffix(a))
			}
		}
	} else {
		pools := groupAgentsByPool(agents)
		for _, pool := range pools {
			_, _ = fmt.Fprintf(w, "  %s %d\n", output.Faint("["+pool.name+"]"), len(pool.agents))
		}
		if total > len(agents) {
			_, _ = fmt.Fprintf(w, "  %s %d more not shown\n", output.Faint("…"), total-len(agents))
		}
	}
}

// agentStatusSuffix flags listed agents that still can't actually run builds.
func agentStatusSuffix(a api.Agent) string {
	var parts []string
	if !a.Connected {
		parts = append(parts, "disconnected")
	}
	if !a.Enabled {
		parts = append(parts, "disabled")
	}
	if !a.Authorized {
		parts = append(parts, "unauthorized")
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + output.Faint("("+strings.Join(parts, ", ")+")")
}

type poolAgents struct {
	name   string
	agents []api.Agent
}

// groupAgentsByPool buckets agents by pool name (stable: pool name asc, then agent name asc).
func groupAgentsByPool(agents []api.Agent) []poolAgents {
	m := map[string][]api.Agent{}
	for _, a := range agents {
		name := "(no pool)"
		if a.Pool != nil && a.Pool.Name != "" {
			name = a.Pool.Name
		}
		m[name] = append(m[name], a)
	}
	out := make([]poolAgents, 0, len(m))
	for name, as := range m {
		slices.SortFunc(as, func(a, b api.Agent) int { return cmp.Compare(a.Name, b.Name) })
		out = append(out, poolAgents{name: name, agents: as})
	}
	slices.SortFunc(out, func(a, b poolAgents) int { return cmp.Compare(a.name, b.name) })
	return out
}

// renderIncompatibilityReasons probes a few agents for unmet requirements; silently skips agents whose reasons REST can't surface.
func renderIncompatibilityReasons(w io.Writer, client api.ClientInterface, buildTypeID string, agents []api.Agent) {
	if buildTypeID == "" || len(agents) == 0 {
		return
	}
	printedHeader := false
	limit := min(reasonProbeAgents, len(agents))
	for i := range limit {
		a := agents[i]
		compat, err := client.GetAgentBuildTypeCompatibility(a.ID, buildTypeID, reasonProbeLimit)
		if err != nil || compat == nil {
			continue
		}
		reasons := compat.ReasonsList()
		if len(reasons) == 0 {
			continue
		}
		if !printedHeader {
			_, _ = fmt.Fprintln(w)
			_, _ = fmt.Fprintf(w, "%s\n", output.Faint("Sample incompatibility reasons:"))
			printedHeader = true
		}
		_, _ = fmt.Fprintf(w, "  %s\n", a.Name)
		for _, r := range reasons {
			_, _ = fmt.Fprintf(w, "    %s %s\n", output.Red("•"), r)
		}
	}
}
