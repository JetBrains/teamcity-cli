package config

import (
	"cmp"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	cfg "github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/output"
	"github.com/spf13/cobra"
)

func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long:  "Get, set, and list CLI configuration values.",
		Args:  cobra.NoArgs,
		RunE:  cmdutil.SubcommandRequired,
	}

	cmd.AddCommand(newListCmd(f))
	cmd.AddCommand(newGetCmd(f))
	cmd.AddCommand(newSetCmd(f))

	return cmd
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List configuration settings",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(f, jsonOutput)
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

type configJSON struct {
	DefaultServer string                `json:"default_server"`
	Servers       map[string]serverJSON `json:"servers"`
	Aliases       map[string]string     `json:"aliases"`
	Environment   map[string]string     `json:"environment,omitempty"`
}

type serverJSON struct {
	Guest       bool   `json:"guest"`
	RO          bool   `json:"ro"`
	TokenExpiry string `json:"token_expiry,omitempty"`
}

func runList(f *cmdutil.Factory, jsonOutput bool) error {
	p := f.Printer
	c := cfg.Get()

	if jsonOutput {
		return printListJSON(p, c)
	}

	_, _ = fmt.Fprintf(p.Out, "%s %s\n\n", output.Faint("Config:"), cfg.ConfigPath())

	if c.DefaultServer != "" {
		_, _ = fmt.Fprintf(p.Out, "default_server=%s\n", c.DefaultServer)
	} else {
		_, _ = fmt.Fprintf(p.Out, "default_server=\n")
	}

	urls := sortedServerURLs(c)
	for _, serverURL := range urls {
		sc := c.Servers[serverURL]
		suffix := ""
		if serverURL == c.DefaultServer && len(urls) > 1 {
			suffix = output.Faint(" (default)")
		}
		_, _ = fmt.Fprintf(p.Out, "\n%s%s\n", serverURL, suffix)
		_, _ = fmt.Fprintf(p.Out, "  guest=%t\n", sc.Guest)
		_, _ = fmt.Fprintf(p.Out, "  ro=%t\n", sc.RO)
		if sc.TokenExpiry != "" {
			_, _ = fmt.Fprintf(p.Out, "  token_expiry=%s\n", sc.TokenExpiry)
		}
	}

	if aliases := cfg.GetAllAliases(); len(aliases) > 0 {
		_, _ = fmt.Fprintf(p.Out, "\n%s %d configured %s\n",
			output.Faint("Aliases:"), len(aliases), output.Faint("(run 'teamcity alias list' to view)"))
	}

	printEnvOverrides(p)
	return nil
}

func printListJSON(p *output.Printer, c *cfg.Config) error {
	servers := map[string]serverJSON{}
	for url, sc := range c.Servers {
		servers[url] = serverJSON{
			Guest:       sc.Guest,
			RO:          sc.RO,
			TokenExpiry: sc.TokenExpiry,
		}
	}
	aliases := c.Aliases
	if aliases == nil {
		aliases = map[string]string{}
	}
	env := collectEnvOverrides()
	out := configJSON{
		DefaultServer: c.DefaultServer,
		Servers:       servers,
		Aliases:       aliases,
	}
	if len(env) > 0 {
		out.Environment = env
	}
	return p.PrintJSON(out)
}

func printEnvOverrides(p *output.Printer) {
	env := collectEnvOverrides()
	if len(env) == 0 {
		return
	}
	_, _ = fmt.Fprintf(p.Out, "\n%s\n", output.Faint("Environment overrides:"))
	for _, key := range slices.Sorted(maps.Keys(env)) {
		_, _ = fmt.Fprintf(p.Out, "  %s %s=%s\n", output.Yellow("!"), key, env[key])
	}
}

func collectEnvOverrides() map[string]string {
	env := map[string]string{}
	for _, key := range []string{cfg.EnvServerURL, cfg.EnvToken, cfg.EnvGuestAuth, cfg.EnvReadOnly} {
		if v := os.Getenv(key); v != "" {
			if key == cfg.EnvToken {
				v = "****"
			}
			env[key] = v
		}
	}
	return env
}

func sortedServerURLs(c *cfg.Config) []string {
	urls := slices.Collect(maps.Keys(c.Servers))
	slices.SortFunc(urls, func(a, b string) int {
		if ad, bd := a == c.DefaultServer, b == c.DefaultServer; ad != bd {
			if ad {
				return -1
			}
			return 1
		}
		return cmp.Compare(a, b)
	})
	return urls
}

func newGetCmd(f *cmdutil.Factory) *cobra.Command {
	var serverURL string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long:  "Get the value of a configuration key.\n\nValid keys: " + strings.Join(cfg.ValidKeys(), ", "),
		Example: `  teamcity config get default_server
  teamcity config get ro
  teamcity config get guest --server tc.example.com`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := cfg.GetField(args[0], serverURL)
			if err != nil {
				return err
			}
			if jsonOutput {
				return f.Printer.PrintJSON(map[string]string{"key": args[0], "value": value})
			}
			_, _ = fmt.Fprintln(f.Printer.Out, value)
			return nil
		},
	}

	cmd.Flags().StringVarP(&serverURL, "server", "s", "", "Server URL for per-server settings")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

func newSetCmd(f *cmdutil.Factory) *cobra.Command {
	var serverURL string

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long:  "Set the value of a configuration key.\n\nValid keys: " + strings.Join(cfg.ValidKeys(), ", "),
		Example: `  # Switch default server
  teamcity config set default_server tc.example.com

  # Enable read-only mode for a server
  teamcity config set ro true --server tc.example.com

  # Enable guest auth for the default server
  teamcity config set guest true`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			if err := cfg.SetField(key, value, serverURL); err != nil {
				return err
			}
			f.Printer.Success("Set %s to %q", key, value)
			return nil
		},
	}

	cmd.Flags().StringVarP(&serverURL, "server", "s", "", "Server URL for per-server settings")
	return cmd
}
