package pipeline

import (
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

const schemaTTL = 24 * time.Hour

//go:embed schema_embedded.json
var embeddedSchema []byte

func schemaDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "tc"), nil
}

func schemaPath(serverURL string) (string, error) {
	dir, err := schemaDir()
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(serverURL))
	return filepath.Join(dir, fmt.Sprintf("pipeline-schema-%x.json", h[:4])), nil
}

func loadCachedSchema(serverURL string) ([]byte, error) {
	path, err := schemaPath(serverURL)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if time.Since(info.ModTime()) > schemaTTL {
		return nil, errors.New("schema cache expired")
	}

	return os.ReadFile(path)
}

func saveSchemaCache(serverURL string, schema []byte) error {
	path, err := schemaPath(serverURL)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	return os.WriteFile(path, schema, 0600)
}

// fetchOrCacheSchema returns the pipeline JSON schema. The bool is true when
// the embedded fallback was returned because the server doesn't support the
// endpoint (e.g. TeamCity < 2026.1). The bool is meaningless when err != nil.
func fetchOrCacheSchema(client *api.Client, refresh bool) ([]byte, bool, error) {
	if !refresh {
		if cached, err := loadCachedSchema(client.BaseURL); err == nil {
			return cached, false, nil
		}
	}

	schema, err := client.GetPipelineSchema()
	if err == nil {
		_ = saveSchemaCache(client.BaseURL, schema)
		return schema, false, nil
	}

	if !refresh {
		return embeddedSchema, true, nil
	}
	return nil, false, fmt.Errorf("failed to fetch pipeline schema from server: %w", err)
}

func newPipelineSchemaCmd(f *cmdutil.Factory) *cobra.Command {
	var refresh bool

	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Print the pipeline JSON schema for the current server",
		Long: `Fetch the per-instance pipeline JSON schema and print it to stdout.

The schema is cached locally for 24 hours. When the server does not support
the schema endpoint (TeamCity < 2026.1), an embedded fallback is printed and
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

			data, fallback, err := fetchOrCacheSchema(c, refresh)
			if err != nil {
				return err
			}
			if fallback {
				_, _ = fmt.Fprintln(f.Printer.ErrOut,
					"warning: server did not return a schema (server may predate TeamCity 2026.1)")
			}
			_, err = f.Printer.Out.Write(data)
			return err
		},
	}

	cmd.Flags().BoolVar(&refresh, "refresh", false, "Force re-fetch from server, bypassing the 24h cache")
	return cmd
}
