package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/JetBrains/teamcity-cli/internal/atomicfile"
)

// ServerInfo is the telemetry-only cache of TC server context, keyed by server URL.
// Lives alongside the FUS buffer in DataDir so users can wipe all telemetry state in one shot.
type ServerInfo struct {
	Version string `json:"version,omitempty"`
	Type    string `json:"type,omitempty"`
}

const serverInfoFile = "server-info.json"

func serverInfoPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, serverInfoFile), nil
}

// LoadServerInfo returns the cached server version + type for serverURL; empty strings when unknown.
func LoadServerInfo(serverURL string) (version, serverType string) {
	path, err := serverInfoPath()
	if err != nil {
		return "", ""
	}
	infos, err := readServerInfo(path)
	if err != nil {
		return "", ""
	}
	info := infos[serverURL]
	return info.Version, info.Type
}

// SaveServerInfo writes the version + type for serverURL, merging into any existing entries.
func SaveServerInfo(serverURL, version, serverType string) error {
	path, err := serverInfoPath()
	if err != nil {
		return err
	}
	infos, _ := readServerInfo(path)
	if infos == nil {
		infos = map[string]ServerInfo{}
	}
	infos[serverURL] = ServerInfo{Version: version, Type: serverType}
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, data)
}

func readServerInfo(path string) (map[string]ServerInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var infos map[string]ServerInfo
	if err := json.Unmarshal(data, &infos); err != nil {
		return nil, err
	}
	return infos, nil
}
