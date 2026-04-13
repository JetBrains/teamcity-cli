package cmdutil

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
)

const schemaCacheTTL = 24 * time.Hour

// FetchOrCachePipelineSchema returns the pipeline JSON schema from a local
// cache (refreshed every 24h), fetches it from the server if stale or missing,
// and falls back to the embedded schema when the server is unreachable.
// Pass refresh=true to force a server fetch regardless of cache age.
func FetchOrCachePipelineSchema(client *api.Client, refresh bool) ([]byte, error) {
	if !refresh {
		if cached, err := loadSchemaCache(client.BaseURL); err == nil {
			return cached, nil
		}
	}

	schema, err := client.GetPipelineSchema()
	if err == nil {
		_ = saveSchemaCache(client.BaseURL, schema)
		return schema, nil
	}

	if refresh {
		return nil, fmt.Errorf("failed to fetch pipeline schema from server: %w", err)
	}
	return pipelineschema.Bytes, nil
}

func schemaCachePath(serverURL string) (string, error) {
	dir, err := config.ConfigDir()
	if err != nil {
		return "", err
	}
	h := sha256.Sum256([]byte(serverURL))
	return filepath.Join(dir, fmt.Sprintf("pipeline-schema-%x.json", h[:4])), nil
}

func loadSchemaCache(serverURL string) ([]byte, error) {
	path, err := schemaCachePath(serverURL)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > schemaCacheTTL {
		return nil, fmt.Errorf("schema cache expired")
	}
	return os.ReadFile(path)
}

func saveSchemaCache(serverURL string, schema []byte) error {
	path, err := schemaCachePath(serverURL)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, schema, 0600)
}
