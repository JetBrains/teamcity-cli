package pipeline

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/JetBrains/teamcity-cli/api"
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
		return nil, fmt.Errorf("schema cache expired")
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

func fetchOrCacheSchema(client *api.Client, refresh bool) ([]byte, error) {
	if !refresh {
		if cached, err := loadCachedSchema(client.BaseURL); err == nil {
			return cached, nil
		}
	}

	schema, err := client.GetPipelineSchema()
	if err == nil {
		_ = saveSchemaCache(client.BaseURL, schema)
		return schema, nil
	}

	if !refresh {
		return embeddedSchema, nil
	}
	return nil, fmt.Errorf("failed to fetch pipeline schema from server: %w", err)
}
