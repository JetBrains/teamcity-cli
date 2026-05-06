package cmdutil

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
	"github.com/JetBrains/teamcity-cli/internal/config"
	"github.com/JetBrains/teamcity-cli/internal/pipelineschema"
)

const schemaCacheTTL = 24 * time.Hour

// FetchOrCachePipelineSchema returns (schema, fromCache, fellBackToEmbedded, err); refresh=true bypasses the 24h cache.
func FetchOrCachePipelineSchema(client *api.Client, refresh bool) ([]byte, bool, bool, error) {
	if !refresh {
		if cached, err := loadSchemaCache(client.BaseURL); err == nil {
			return cached, true, false, nil
		}
	}

	schema, err := client.GetPipelineSchema()
	if err == nil {
		_ = saveSchemaCache(client.BaseURL, schema)
		return schema, false, false, nil
	}

	if !refresh && errors.Is(err, api.ErrPipelineSchemaUnsupported) {
		return pipelineschema.Bytes, false, true, nil
	}
	return nil, false, false, fmt.Errorf("failed to fetch pipeline schema from server: %w", err)
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
		return nil, errors.New("schema cache expired")
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
