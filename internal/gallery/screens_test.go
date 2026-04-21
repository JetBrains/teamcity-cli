//go:build gallery

package gallery_test

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/tiulpin/termbook"

	"github.com/JetBrains/teamcity-cli/internal/cmdtest"
	"github.com/JetBrains/teamcity-cli/internal/output"
)

func styleGuideScreens() []termbook.Screen {
	return []termbook.Screen{
		termbook.Manual("colors", "Colors & Text Styles", "Standard color palette", "", func(w io.Writer) {
			fmt.Fprintln(w, output.Green("Green")+" — success, links, positive")
			fmt.Fprintln(w, output.Red("Red")+" — errors, failures, negative")
			fmt.Fprintln(w, output.Yellow("Yellow")+" — warnings, running, caution")
			fmt.Fprintln(w, output.Cyan("Cyan")+" — titles, names, emphasis")
			fmt.Fprintln(w, output.Bold("Bold")+" — headers, key labels")
			fmt.Fprintln(w, output.Faint("Faint")+" — secondary info, IDs, hints")
		}),
		termbook.Manual("status-icons", "Status Icons", "Status indicators in tables and views", "", func(w io.Writer) {
			for _, s := range []struct{ st, state, text, desc string }{
				{"SUCCESS", "finished", "", "build completed successfully"},
				{"FAILURE", "finished", "", "build finished with failures"},
				{"", "running", "", "build is currently executing"},
				{"", "queued", "", "build is waiting in queue"},
				{"UNKNOWN", "finished", "Canceled", "build was canceled by a user"},
				{"ERROR", "finished", "", "internal or infrastructure error"},
				{"UNKNOWN", "finished", "", "unknown status"},
			} {
				fmt.Fprintf(w, "  %s  %-12s %s\n",
					output.StatusIcon(s.st, s.state, s.text),
					output.StatusText(s.st, s.state, s.text),
					output.Faint(s.desc))
			}
		}),
		termbook.Manual("messages", "Messages", "Info, success, warning, tip, error patterns", "", func(w io.Writer) {
			p := &output.Printer{Out: w, ErrOut: w}
			p.Success("Logged in as Viktor Tiulpin")
			p.Info("3 runs matched your filters")
			p.Warn("Token expires in 2 days")
			p.Tip("Use --limit to show more runs")
			fmt.Fprintf(w, "Error: job %q not found\n\n%s\n", "NonExistent_Build", output.FormatTip("Run 'teamcity job list' to see available jobs"))
		}),
		termbook.Manual("table", "Table Rendering", "Auto-sized columns with colored cells", "", func(w io.Writer) {
			p := &output.Printer{Out: w, ErrOut: w}
			p.PrintTable([]string{"ID", "NAME", "STATUS", "POOL"}, [][]string{
				{"1", "linux-agent-01", output.Green("Connected"), "Default"},
				{"2", "linux-agent-02", output.Green("Connected"), "Default"},
				{"3", "mac-agent-01", output.Red("Disconnected"), "macOS"},
				{"4", "cloud-agent-01", output.Yellow("Disabled"), "Cloud"},
			})
		}),
		termbook.Manual("tree", "Tree Rendering", "Hierarchical display with connectors", "", func(w io.Writer) {
			p := &output.Printer{Out: w, ErrOut: w}
			p.PrintTree(output.TreeNode{
				Label: output.Cyan("My Application") + " " + output.Faint("MyApp"),
				Children: []output.TreeNode{
					{Label: output.Cyan("Build") + " " + output.Faint("MyApp_Build")},
					{Label: output.Cyan("Run Tests") + " " + output.Faint("MyApp_Test")},
					{Label: output.Cyan("Deploy") + " " + output.Faint("MyApp_Deploy"), Children: []output.TreeNode{
						{Label: output.Cyan("Smoke Tests") + " " + output.Faint("MyApp_Smoke")},
					}},
				},
			})
		}),
	}
}
func helpScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	h := func(id, title, desc, cmd string, args ...string) termbook.Screen {
		return termbook.Scr(id, title, desc, "teamcity "+cmd+" --help", capture(t, ts, args...))
	}
	return []termbook.Screen{
		h("help-root", "teamcity", "Root command — shows logo and common commands", "", "--help"),
		h("help-run", "run", "Run (build) management commands", "run", "run", "--help"),
		h("help-job", "job", "Job (build configuration) commands", "job", "job", "--help"),
		h("help-agent", "agent", "Build agent commands", "agent", "agent", "--help"),
		h("help-queue", "queue", "Build queue commands", "queue", "queue", "--help"),
		h("help-pool", "pool", "Agent pool commands", "pool", "pool", "--help"),
		h("help-project", "project", "Project management commands", "project", "project", "--help"),
		h("help-pipeline", "pipeline", "YAML pipeline commands", "pipeline", "pipeline", "--help"),
		h("help-auth", "auth", "Authentication commands", "auth", "auth", "--help"),
		h("help-config", "config", "Configuration commands", "config", "config", "--help"),
		h("help-alias", "alias", "Command alias management", "alias", "alias", "--help"),
		h("help-skill", "skill", "AI agent skill management", "skill", "skill", "--help"),
		h("help-project-settings", "project settings", "Versioned settings subcommands", "project settings", "project", "settings", "--help"),
		h("help-project-vcs", "project vcs", "VCS root subcommands", "project vcs", "project", "vcs", "--help"),
		h("help-project-ssh", "project ssh", "SSH key subcommands", "project ssh", "project", "ssh", "--help"),
		h("help-project-connection", "project connection", "Project connection subcommands", "project connection", "project", "connection", "--help"),
		h("help-project-cloud", "project cloud", "Cloud management subcommands", "project cloud", "project", "cloud", "--help"),
		h("help-project-cloud-profile", "project cloud profile", "Cloud profile subcommands", "project cloud profile", "project", "cloud", "profile", "--help"),
		h("help-project-cloud-image", "project cloud image", "Cloud image subcommands", "project cloud image", "project", "cloud", "image", "--help"),
		h("help-project-cloud-instance", "project cloud instance", "Cloud instance subcommands", "project cloud instance", "project", "cloud", "instance", "--help"),
		h("help-project-token", "project token", "Secure token subcommands", "project token", "project", "token", "--help"),
		h("help-project-param", "project param", "Project parameter subcommands", "project param", "project", "param", "--help"),
		h("help-job-param", "job param", "Job parameter subcommands", "job param", "job", "param", "--help"),
	}
}
func runScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("run-list", "run list", "List recent runs with status, job, branch, and timing",
			"teamcity run list", capture(t, ts, "run", "list")),
		termbook.Scr("run-view", "run view", "Detail view of a finished run",
			"teamcity run view 45231", capture(t, ts, "run", "view", "45231")),
		termbook.Scr("run-start", "run start", "Trigger a new run",
			"teamcity run start MyApp_Build --no-input", capture(t, ts, "run", "start", "MyApp_Build", "--no-input")),
		termbook.Scr("run-restart", "run restart", "Re-trigger a completed run",
			"teamcity run restart 1 --no-input", capture(t, ts, "run", "restart", "1", "--no-input")),
		termbook.Manual("run-watch", "run watch --logs", "Full-screen TUI with live log streaming (alt-screen)", "teamcity run watch --logs 45229", func(w io.Writer) {
			fmt.Fprintf(w, "%s %s 45229  #830 %s · Running (67%%)\n\n",
				output.Yellow("●"),
				output.Bold("Deploy Staging"),
				output.Faint("https://tc.example.com/viewLog.html?buildId=45229"))
			fmt.Fprintf(w, "%s\n", output.Faint("[12:01:10] Downloading artifacts from Build #831"))
			fmt.Fprintf(w, "%s\n", output.Faint("[12:01:12] Extracting app.jar (12.4 MB)"))
			fmt.Fprintf(w, "%s\n", output.Faint("[12:01:15] Verifying checksums... OK"))
			fmt.Fprintf(w, "[12:01:18] Starting: deploy.sh --env staging\n")
			fmt.Fprintf(w, "[12:01:22] Stopping existing service...\n")
			fmt.Fprintf(w, "[12:01:25] Deploying version 1.0.0 to staging-01\n")
			fmt.Fprintf(w, "[12:01:30] Deploying version 1.0.0 to staging-02\n")
			fmt.Fprintf(w, "%s\n", output.Green("[12:01:35] Health check staging-01: 200 OK"))
			fmt.Fprintf(w, "%s\n", output.Green("[12:01:38] Health check staging-02: 200 OK"))
			fmt.Fprintf(w, "[12:01:40] Updating load balancer config...\n")
			fmt.Fprintf(w, "[12:01:42] Draining old instances...\n")
			fmt.Fprintf(w, "%s\n", output.Green("[12:01:45] Deploy complete. 2/2 instances healthy"))
			fmt.Fprintf(w, "\n%s  %s  %s\n",
				output.Faint("q quit"),
				output.Faint("·"),
				output.Cyan("teamcity agent term 1"))
		}),
		termbook.Scr("run-log", "run log --tail", "Formatted build log with timestamps",
			"teamcity run log 1 --tail 10", capture(t, ts, "run", "log", "1", "--tail", "10")),
		termbook.Scr("run-diff", "run diff", "Compare two runs side by side",
			"teamcity run diff 45231 45230", capture(t, ts, "run", "diff", "45231", "45230")),
		termbook.Scr("run-artifacts", "run artifacts", "List downloadable build artifacts",
			"teamcity run artifacts 1", capture(t, ts, "run", "artifacts", "1")),
		termbook.Manual("run-download", "run download", "Download artifacts to local filesystem", "teamcity run download 45231", func(w io.Writer) {
			fmt.Fprintln(w, "Downloading 3 artifacts (12.6 MiB total) to ./artifacts/")
			fmt.Fprintln(w)
			fmt.Fprintf(w, "%-18s  %10s\n", "NAME", "SIZE")
			fmt.Fprintf(w, "%-18s  %10s  %s\n", "app.jar", "12 MiB", output.Green("   ✓"))
			fmt.Fprintf(w, "%-18s  %10s  %s\n", "test-report.html", "234 KiB", output.Green("   ✓"))
			fmt.Fprintf(w, "%-18s  %10s  %s\n", "logs/build.log", "45 KiB", output.Green("   ✓"))
			fmt.Fprintf(w, "\n%s 3 artifacts downloaded\n", output.Green("✓"))
		}),
		termbook.Scr("run-changes", "run changes", "VCS changes included in a run",
			"teamcity run changes 1", capture(t, ts, "run", "changes", "1")),
		termbook.Scr("run-tests", "run tests", "Test results summary",
			"teamcity run tests 1", capture(t, ts, "run", "tests", "1")),
		termbook.Scr("run-tree", "run tree", "Snapshot dependency tree of a run",
			"teamcity run tree 45229", capture(t, ts, "run", "tree", "45229")),
		termbook.Scr("run-cancel", "run cancel", "Cancel a running or queued build",
			"teamcity run cancel --yes 45233", capture(t, ts, "run", "cancel", "--yes", "45233")),
		termbook.Scr("run-pin", "run pin", "Pin a build to prevent cleanup",
			"teamcity run pin 1 -m \"Release candidate\"", capture(t, ts, "run", "pin", "1", "-m", "Release candidate")),
		termbook.Scr("run-unpin", "run unpin", "Unpin a build",
			"teamcity run unpin 1", capture(t, ts, "run", "unpin", "1")),
		termbook.Scr("run-tag", "run tag", "Add tags to a run",
			"teamcity run tag 1 release v1.0.0", capture(t, ts, "run", "tag", "1", "release", "v1.0.0")),
		termbook.Scr("run-untag", "run untag", "Remove tags from a run",
			"teamcity run untag 1 release", capture(t, ts, "run", "untag", "1", "release")),
		termbook.Scr("run-comment", "run comment", "View a build comment",
			"teamcity run comment 1", capture(t, ts, "run", "comment", "1")),
		termbook.Scr("run-comment-set", "run comment (set)", "Set a comment on a run",
			"teamcity run comment 1 \"Ready for prod\"", capture(t, ts, "run", "comment", "1", "Ready for prod")),
	}
}
func jobScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("job-list", "job list", "List build configurations", "teamcity job list", capture(t, ts, "job", "list")),
		termbook.Scr("job-view", "job view", "Detail view of a build configuration",
			"teamcity job view MyApp_Build", capture(t, ts, "job", "view", "MyApp_Build")),
		termbook.Scr("job-tree", "job tree", "Snapshot dependency tree of a job",
			"teamcity job tree MyApp_Build", capture(t, ts, "job", "tree", "MyApp_Build")),
		termbook.Scr("job-pause", "job pause", "Pause a build configuration",
			"teamcity job pause MyApp_Deploy", capture(t, ts, "job", "pause", "MyApp_Deploy")),
		termbook.Scr("job-resume", "job resume", "Resume a paused build configuration",
			"teamcity job resume MyApp_Deploy", capture(t, ts, "job", "resume", "MyApp_Deploy")),
		termbook.Scr("job-param", "job param list", "List job parameters",
			"teamcity job param list MyApp_Build", capture(t, ts, "job", "param", "list", "MyApp_Build")),
		termbook.Scr("job-param-get", "job param get", "Get a job parameter value",
			"teamcity job param get MyApp_Build env.JAVA_HOME", capture(t, ts, "job", "param", "get", "MyApp_Build", "env.JAVA_HOME")),
		termbook.Scr("job-param-set", "job param set", "Set a job parameter value",
			"teamcity job param set MyApp_Build env.JAVA_HOME /opt/jdk", capture(t, ts, "job", "param", "set", "MyApp_Build", "env.JAVA_HOME", "/opt/jdk")),
		termbook.Scr("job-param-delete", "job param delete", "Delete a job parameter",
			"teamcity job param delete MyApp_Build env.JAVA_HOME", capture(t, ts, "job", "param", "delete", "MyApp_Build", "env.JAVA_HOME")),
	}
}
func agentScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("agent-list", "agent list", "List build agents", "teamcity agent list", capture(t, ts, "agent", "list")),
		termbook.Scr("agent-view", "agent view", "Detailed view of an agent",
			"teamcity agent view 1", capture(t, ts, "agent", "view", "1")),
		termbook.Scr("agent-jobs", "agent jobs", "Compatible and incompatible jobs",
			"teamcity agent jobs 1", capture(t, ts, "agent", "jobs", "1")),
		termbook.Scr("agent-enable", "agent enable", "Enable an agent",
			"teamcity agent enable 1", capture(t, ts, "agent", "enable", "1")),
		termbook.Scr("agent-disable", "agent disable", "Disable an agent",
			"teamcity agent disable 1", capture(t, ts, "agent", "disable", "1")),
		termbook.Scr("agent-authorize", "agent authorize", "Authorize an agent to accept builds",
			"teamcity agent authorize 1", capture(t, ts, "agent", "authorize", "1")),
		termbook.Scr("agent-deauthorize", "agent deauthorize", "Revoke authorization from an agent",
			"teamcity agent deauthorize 1", capture(t, ts, "agent", "deauthorize", "1")),
		termbook.Scr("agent-move", "agent move", "Move an agent to a different pool",
			"teamcity agent move 1 2", capture(t, ts, "agent", "move", "1", "2")),
		termbook.Scr("agent-reboot", "agent reboot", "Request agent reboot",
			"teamcity agent reboot --yes 1", capture(t, ts, "agent", "reboot", "--yes", "1")),
		termbook.Manual("agent-term", "agent term", "Interactive terminal session on an agent (WebSocket)", "teamcity agent term linux-agent-01", func(w io.Writer) {
			fmt.Fprintf(w, "%s Connected to %s\n", output.Green("✓"), output.Cyan("linux-agent-01"))
			fmt.Fprintf(w, "%s\n\n", output.Faint("https://tc.example.com/agentDetails.html?id=1"))
			fmt.Fprintf(w, "%s uname -a\n", output.Green("builduser@linux-agent-01:~$"))
			fmt.Fprintln(w, "Linux linux-agent-01 5.15.0-1056-aws #61-Ubuntu SMP x86_64 GNU/Linux")
			fmt.Fprintf(w, "%s df -h /opt/buildagent\n", output.Green("builduser@linux-agent-01:~$"))
			fmt.Fprintln(w, "Filesystem      Size  Used Avail Use% Mounted on")
			fmt.Fprintln(w, "/dev/nvme1n1    200G   87G  113G  44% /opt/buildagent")
			fmt.Fprintf(w, "%s docker ps --format '{{.Names}}  {{.Status}}'\n", output.Green("builduser@linux-agent-01:~$"))
			fmt.Fprintln(w, "tc-build-45229  Up 3 minutes")
			fmt.Fprintln(w, "tc-build-45231  Up 12 minutes")
			fmt.Fprintf(w, "%s free -h | head -2\n", output.Green("builduser@linux-agent-01:~$"))
			fmt.Fprintln(w, "              total        used        free      shared  buff/cache   available")
			fmt.Fprintln(w, "Mem:           31Gi        18Gi       2.1Gi       312Mi        11Gi        12Gi")
			fmt.Fprintf(w, "%s ", output.Green("builduser@linux-agent-01:~$"))
		}),
		termbook.Manual("agent-exec", "agent exec", "Execute a single command on an agent (WebSocket)", "teamcity agent exec linux-agent-01 -- top -bn1 | head -5", func(w io.Writer) {
			fmt.Fprintln(w, "top - 19:31:45 up 14 days,  3:22,  0 users,  load average: 4.12, 3.87, 3.54")
			fmt.Fprintln(w, "Tasks: 287 total,   3 running, 284 sleeping,   0 stopped,   0 zombie")
			fmt.Fprintln(w, "%Cpu(s): 42.3 us,  5.1 sy,  0.0 ni, 51.2 id,  0.8 wa,  0.0 hi,  0.6 si")
			fmt.Fprintln(w, "MiB Mem :  32168.4 total,   2152.3 free,  18724.1 used,  11292.0 buff/cache")
			fmt.Fprintln(w, "MiB Swap:   8192.0 total,   8192.0 free,      0.0 used.  12876.4 avail Mem")
		}),
	}
}
func queueScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("queue-list", "queue list", "List builds in the queue", "teamcity queue list", capture(t, ts, "queue", "list")),
		termbook.Scr("queue-remove", "queue remove", "Remove a build from the queue",
			"teamcity queue remove --yes 100", capture(t, ts, "queue", "remove", "--yes", "100")),
		termbook.Scr("queue-top", "queue top", "Move a build to the top of the queue",
			"teamcity queue top 100", capture(t, ts, "queue", "top", "100")),
		termbook.Scr("queue-approve", "queue approve", "Approve a queued build",
			"teamcity queue approve 100", capture(t, ts, "queue", "approve", "100")),
	}
}
func poolScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("pool-list", "pool list", "List agent pools", "teamcity pool list", capture(t, ts, "pool", "list")),
		termbook.Scr("pool-view", "pool view", "Detailed pool with agents and projects",
			"teamcity pool view 0", capture(t, ts, "pool", "view", "0")),
		termbook.Scr("pool-link", "pool link", "Link a project to a pool",
			"teamcity pool link 0 MyApp", capture(t, ts, "pool", "link", "0", "MyApp")),
		termbook.Scr("pool-unlink", "pool unlink", "Unlink a project from a pool",
			"teamcity pool unlink 0 MyApp", capture(t, ts, "pool", "unlink", "0", "MyApp")),
	}
}
func projectScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("project-list", "project list", "List projects", "teamcity project list", capture(t, ts, "project", "list")),
		termbook.Scr("project-view", "project view", "Detailed project view",
			"teamcity project view MyApp", capture(t, ts, "project", "view", "MyApp")),
		termbook.Scr("project-tree", "project tree", "Full hierarchy with jobs and pipelines",
			"teamcity project tree", capture(t, ts, "project", "tree")),
		termbook.Scr("project-settings", "project settings status", "Versioned settings sync status",
			"teamcity project settings status MyApp", capture(t, ts, "project", "settings", "status", "MyApp")),
		termbook.Manual("project-settings-validate", "project settings validate", "Validate local DSL settings", "teamcity project settings validate .teamcity", func(w io.Writer) {
			fmt.Fprintf(w, "Validating %s\n", output.Faint(".teamcity"))
			fmt.Fprintf(w, "%s Configuration valid\n", output.Green("✓"))
			fmt.Fprintf(w, "  %s %s\n", output.Faint("Server:"), "https://tc.example.com")
			fmt.Fprintf(w, "  %s\n", output.Faint("Projects: 5, Build configurations: 12"))
		}),
		termbook.Manual("project-settings-export", "project settings export", "Export DSL settings as a zip archive", "teamcity project settings export MyApp", func(w io.Writer) {
			fmt.Fprintln(w, "Exported kotlin settings to projectSettings.zip (45678 bytes)")
		}),
		termbook.Scr("project-vcs-list", "project vcs list", "List VCS roots in a project",
			"teamcity project vcs list --project MyApp", capture(t, ts, "project", "vcs", "list", "--project", "MyApp")),
		termbook.Scr("project-vcs-view", "project vcs view", "View VCS root details",
			"teamcity project vcs view MyApp_MainRepo", capture(t, ts, "project", "vcs", "view", "MyApp_MainRepo")),
		termbook.Manual("project-vcs-create", "project vcs create", "Create a new VCS root (interactive prompts)", "teamcity project vcs create --project MyApp --name \"New Repo\" --url https://github.com/org/new-repo", func(w io.Writer) {
			fmt.Fprintf(w, "%s Created VCS root \"New Repo\" (MyApp_NewRepo) in project MyApp\n", output.Green("✓"))
		}),
		termbook.Scr("project-vcs-test", "project vcs test", "Test VCS connection",
			"teamcity project vcs test MyApp_MainRepo", capture(t, ts, "project", "vcs", "test", "MyApp_MainRepo")),
		termbook.Scr("project-vcs-delete", "project vcs delete", "Delete a VCS root",
			"teamcity project vcs delete --yes MyApp_Repo", capture(t, ts, "project", "vcs", "delete", "--yes", "MyApp_Repo")),
		termbook.Scr("project-ssh-list", "project ssh list", "List SSH keys in a project",
			"teamcity project ssh list --project MyApp", capture(t, ts, "project", "ssh", "list", "--project", "MyApp")),
		termbook.Scr("project-ssh-generate", "project ssh generate", "Generate a new SSH key",
			"teamcity project ssh generate --project MyApp --name ci-key", capture(t, ts, "project", "ssh", "generate", "--project", "MyApp", "--name", "ci-key")),
		termbook.Manual("project-ssh-upload", "project ssh upload", "Upload an SSH private key", "teamcity project ssh upload id_ed25519 --project MyApp --name deploy-key", func(w io.Writer) {
			fmt.Fprintf(w, "%s Uploaded SSH key \"deploy-key\" to project MyApp\n", output.Green("✓"))
		}),
		termbook.Scr("project-ssh-delete", "project ssh delete", "Delete an SSH key",
			"teamcity project ssh delete deploy-key --project MyApp", capture(t, ts, "project", "ssh", "delete", "deploy-key", "--project", "MyApp")),
		termbook.Scr("project-connection-list", "project connection list", "List project connections",
			"teamcity project connection list --project MyApp", capture(t, ts, "project", "connection", "list", "--project", "MyApp")),
		termbook.Scr("project-cloud-profile", "project cloud profile list", "List cloud profiles",
			"teamcity project cloud profile list", capture(t, ts, "project", "cloud", "profile", "list")),
		termbook.Scr("project-cloud-profile-view", "project cloud profile view", "View cloud profile details",
			"teamcity project cloud profile view aws-prod", capture(t, ts, "project", "cloud", "profile", "view", "aws-prod")),
		termbook.Scr("project-cloud-image", "project cloud image list", "List cloud images",
			"teamcity project cloud image list", capture(t, ts, "project", "cloud", "image", "list")),
		termbook.Scr("project-cloud-image-view", "project cloud image view", "View cloud image details",
			"teamcity project cloud image view img-1", capture(t, ts, "project", "cloud", "image", "view", "id:img-1,profileId:aws-prod")),
		termbook.Scr("project-cloud-image-start", "project cloud image start", "Start a cloud instance from an image",
			"teamcity project cloud image start img-1", capture(t, ts, "project", "cloud", "image", "start", "id:img-1,profileId:aws-prod")),
		termbook.Scr("project-cloud-instance", "project cloud instance list", "List running cloud instances",
			"teamcity project cloud instance list", capture(t, ts, "project", "cloud", "instance", "list")),
		termbook.Scr("project-cloud-instance-view", "project cloud instance view", "View cloud instance details",
			"teamcity project cloud instance view i-024...", capture(t, ts, "project", "cloud", "instance", "view", "i-0245b46070c443201")),
		termbook.Scr("project-cloud-instance-stop", "project cloud instance stop", "Stop a cloud instance",
			"teamcity project cloud instance stop i-024...", capture(t, ts, "project", "cloud", "instance", "stop", "i-0245b46070c443201")),
		termbook.Scr("project-param", "project param list", "List project parameters",
			"teamcity project param list MyApp", capture(t, ts, "project", "param", "list", "MyApp")),
		termbook.Scr("project-param-get", "project param get", "Get a project parameter value",
			"teamcity project param get MyApp env.DEPLOY_ENV", capture(t, ts, "project", "param", "get", "MyApp", "env.DEPLOY_ENV")),
		termbook.Scr("project-param-set", "project param set", "Set a project parameter value",
			"teamcity project param set MyApp env.DEPLOY_ENV prod", capture(t, ts, "project", "param", "set", "MyApp", "env.DEPLOY_ENV", "prod")),
		termbook.Scr("project-param-delete", "project param delete", "Delete a project parameter",
			"teamcity project param delete MyApp env.DEPLOY_ENV", capture(t, ts, "project", "param", "delete", "MyApp", "env.DEPLOY_ENV")),
		termbook.Scr("project-token-put", "project token put", "Store a secret and get a secure token reference",
			"teamcity project token put MyApp \"s3cret-db-password\"", capture(t, ts, "project", "token", "put", "MyApp", "s3cret-db-password")),
		termbook.Manual("project-token-get", "project token get", "Retrieve the value of a secure token", "teamcity project token get MyApp credentialsJSON:abc123", func(w io.Writer) {
			fmt.Fprintln(w, "s3cret-db-password")
		}),
	}
}
func pipelineScreens(t *testing.T, ts *cmdtest.TestServer, yamlPath string) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("pipeline-list", "pipeline list", "List YAML pipelines", "teamcity pipeline list", capture(t, ts, "pipeline", "list")),
		termbook.Scr("pipeline-view", "pipeline view", "View pipeline details and jobs",
			"teamcity pipeline view MyApp_CI", capture(t, ts, "pipeline", "view", "MyApp_CI")),
		termbook.Scr("pipeline-create", "pipeline create", "Create a new pipeline from YAML",
			"teamcity pipeline create Onboarding --project MyApp --vcs-root MyApp_MainRepo --file .teamcity.yml",
			capture(t, ts, "pipeline", "create", "Onboarding", "--project", "MyApp", "--vcs-root", "MyApp_MainRepo", "--file", yamlPath)),
		termbook.Scr("pipeline-validate", "pipeline validate", "Validate pipeline YAML",
			"teamcity pipeline validate .teamcity.yml",
			strings.ReplaceAll(capture(t, ts, "pipeline", "validate", yamlPath), yamlPath, ".teamcity.yml")),
		termbook.Scr("pipeline-delete", "pipeline delete", "Delete a pipeline",
			"teamcity pipeline delete --yes MyApp_CI", capture(t, ts, "pipeline", "delete", "--yes", "MyApp_CI")),
		termbook.Manual("pipeline-push", "pipeline push", "Upload YAML to a pipeline", "teamcity pipeline push MyApp_CI .teamcity.yml", func(w io.Writer) {
			fmt.Fprintf(w, "%s Updated pipeline MyApp_CI\n", output.Green("✓"))
		}),
		termbook.Manual("pipeline-pull", "pipeline pull", "Download pipeline YAML", "teamcity pipeline pull MyApp_CI", func(w io.Writer) {
			fmt.Fprintf(w, "%s Written to .teamcity.yml\n", output.Green("✓"))
		}),
	}
}
func authScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("auth-status", "auth status", "Authentication status",
			"teamcity auth status", capture(t, ts, "auth", "status")),
		termbook.Manual("auth-status-multi", "auth status (multi-server)", "Multi-server authentication status", "teamcity auth status", func(w io.Writer) {
			fmt.Fprintf(w, "%s Logged in to %s %s\n", output.Green("✓"), output.Cyan("https://tc.example.com"), output.Faint("(default)"))
			fmt.Fprintf(w, "  %s Viktor Tiulpin (vtiulpin) %s %s\n", output.Faint("User:"), output.Faint("·"), output.Faint("system keyring"))
			fmt.Fprintln(w, "  Token expires: Dec 31, 2026")
			fmt.Fprintf(w, "  %s\n", output.Faint("Server: TeamCity 2025.7 (build 197398)"))
			fmt.Fprintf(w, "  %s %s\n", output.Green("✓"), output.Faint("API compatible"))
			fmt.Fprintln(w)
			fmt.Fprintf(w, "%s Guest access to %s\n", output.Green("✓"), output.Cyan("https://staging.tc.example.com"))
			fmt.Fprintf(w, "  %s\n", output.Faint("Server: TeamCity 2025.7 (build 197398)"))
			fmt.Fprintf(w, "  %s %s\n", output.Green("✓"), output.Faint("API compatible"))
			fmt.Fprintln(w)
			fmt.Fprintf(w, "%s Logged in to %s\n", output.Green("✓"), output.Cyan("https://legacy.tc.example.com"))
			fmt.Fprintf(w, "  %s Viktor Tiulpin (vtiulpin) %s %s\n", output.Faint("User:"), output.Faint("·"), output.Faint("system keyring"))
			fmt.Fprintf(w, "  %s Token expired on Mar 15, 2025\n", output.Red("✗"))
			fmt.Fprintf(w, "  Run %s to re-authenticate\n", output.Cyan("teamcity auth login"))
			fmt.Fprintf(w, "  %s\n", output.Faint("Server: TeamCity 2024.3 (build 185432)"))
			fmt.Fprintf(w, "  %s CLI requires TeamCity 2024.7 or later\n", output.Yellow("!"))
		}),
		termbook.Manual("auth-login", "auth login", "Interactive authentication flow (browser-based PKCE)", "teamcity auth login", func(w io.Writer) {
			fmt.Fprintln(w, "Secure browser login available on this server")
			fmt.Fprintln(w, "Opening browser for authentication...")
			fmt.Fprintf(w, "  %s Approve access in TeamCity\n\n", output.Yellow("→"))
			fmt.Fprintln(w, output.Green("✓"))
			fmt.Fprintf(w, "%s Logged in as %s\n", output.Green("✓"), output.Cyan("Viktor Tiulpin"))
			fmt.Fprintf(w, "%s Token stored in system keyring\n", output.Green("✓"))
			fmt.Fprintf(w, "Token expires: %s\n", output.Yellow("Dec 31, 2026"))
		}),
		termbook.Scr("auth-logout", "auth logout", "Log out from a server",
			"teamcity auth logout", capture(t, ts, "auth", "logout")),
	}
}
func configScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("config-list", "config list", "Show full CLI configuration",
			"teamcity config list", capture(t, ts, "config", "list")),
		termbook.Scr("config-get", "config get", "Get a config value",
			"teamcity config get default_server", capture(t, ts, "config", "get", "default_server")),
		termbook.Manual("config-set", "config set", "Set a config value", "teamcity config set default_server https://tc.example.com", func(w io.Writer) {
			fmt.Fprintf(w, "%s Set default_server to %q\n", output.Green("✓"), "https://tc.example.com")
		}),
	}
}
func aliasScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("alias-list", "alias list", "List configured command aliases",
			"teamcity alias list", capture(t, ts, "alias", "list")),
		termbook.Scr("alias-set", "alias set", "Create or update an alias",
			"teamcity alias set mybuilds \"run list\"", capture(t, ts, "alias", "set", "mybuilds", "run list")),
		termbook.Scr("alias-delete", "alias delete", "Remove an alias",
			"teamcity alias delete mybuilds", capture(t, ts, "alias", "delete", "mybuilds")),
	}
}
func skillScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("skill-list", "skill list", "List available AI agent skills",
			"teamcity skill list", capture(t, ts, "skill", "list")),
		termbook.Scr("skill-install", "skill install", "Install a skill",
			"teamcity skill install teamcity-cli", capture(t, ts, "skill", "install", "teamcity-cli")),
		termbook.Scr("skill-update", "skill update", "Update installed skills",
			"teamcity skill update", capture(t, ts, "skill", "update")),
		termbook.Scr("skill-remove", "skill remove", "Remove an installed skill",
			"teamcity skill remove teamcity-cli", capture(t, ts, "skill", "remove", "teamcity-cli")),
	}
}
func apiScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("api-get", "api", "Make raw REST API requests",
			"teamcity api /app/rest/server", capture(t, ts, "api", "/app/rest/server")),
	}
}
func updateScreens(t *testing.T, ts *cmdtest.TestServer) []termbook.Screen {
	return []termbook.Screen{
		termbook.Scr("update", "update", "Check for CLI updates",
			"teamcity update", capture(t, ts, "update")),
	}
}
func errorScreens() []termbook.Screen {
	return []termbook.Screen{
		termbook.Manual("error-not-found", "Not Found", "Resource not found with contextual hint", "", func(w io.Writer) {
			fmt.Fprintf(w, "Error: run %q not found\n\n%s\n", "999999", output.FormatTip("Run 'teamcity run list' to see available runs"))
		}),
		termbook.Manual("error-auth", "Authentication Failed", "Invalid or expired token", "", func(w io.Writer) {
			fmt.Fprintf(w, "Error: authentication failed: invalid or expired credentials\n\n%s\n", output.FormatTip("Run 'teamcity auth login' to re-authenticate"))
		}),
		termbook.Manual("error-permission", "Permission Denied", "Insufficient permissions", "", func(w io.Writer) {
			fmt.Fprintf(w, "Error: missing %q permission\n\n%s\n", "Manage agents", output.FormatTip("Ask your TeamCity administrator to grant this permission"))
		}),
		termbook.Manual("error-network", "Network Error", "Cannot reach the server", "", func(w io.Writer) {
			fmt.Fprintf(w, "Error: cannot connect to https://tc.example.com: dial tcp: lookup tc.example.com: no such host\n\n%s\n", output.FormatTip("Check your network connection and verify the server URL"))
		}),
		termbook.Manual("error-readonly", "Read-Only Mode", "Write operation blocked by TEAMCITY_RO", "", func(w io.Writer) {
			fmt.Fprintf(w, "Error: read-only mode: write operations are not allowed: POST /app/rest/buildQueue\n\n%s\n", output.FormatTip("Unset the TEAMCITY_RO environment variable to allow write operations"))
		}),
		termbook.Manual("error-json", "JSON Error Format", "Structured error with --json flag", "", func(w io.Writer) {
			fmt.Fprintln(w, `{`)
			fmt.Fprintln(w, `  "error": {`)
			fmt.Fprintln(w, `    "code": "not_found",`)
			fmt.Fprintln(w, `    "message": "run \"999999\" not found",`)
			fmt.Fprintln(w, `    "suggestion": "Run 'teamcity run list' to see available runs"`)
			fmt.Fprintln(w, `  }`)
			fmt.Fprintln(w, `}`)
		}),
	}
}
