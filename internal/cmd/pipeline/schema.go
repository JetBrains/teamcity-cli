package pipeline

import (
	"errors"
	"fmt"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newPipelineSchemaCmd(f *cmdutil.Factory) *cobra.Command {
	var refresh bool

	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Print the pipeline JSON schema for the current server",
		Long: `Fetch the complete pipeline JSON schema, including enabled runners and features,
and print it to stdout.

The schema is cached locally for 24 hours. When the server does not support
the complete schema endpoint, an embedded fallback is printed and
a warning is written to stderr; pass --refresh to require a live server fetch.`,
		Example: `  teamcity pipeline schema
  teamcity pipeline schema > schema.json
  teamcity pipeline schema --refresh`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.Client()
			if err != nil {
				return err
			}
			c, ok := client.(*api.Client)
			if !ok {
				return errors.New("schema requires a real API client")
			}

			data, _, fallback, err := cmdutil.FetchOrCachePipelineSchema(c, refresh)
			if err != nil {
				return err
			}
			if fallback {
				_, _ = fmt.Fprintln(f.Printer.ErrOut,
					"warning: complete schema endpoint is unavailable; using limited embedded schema")
			}
			_, err = f.Printer.Out.Write(data)
			return err
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force re-fetch from server, bypassing the 24h cache")
	return cmd
}
