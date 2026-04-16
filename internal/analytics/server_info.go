package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	return atomicWrite(path, data)
}

// atomicWrite stages bytes to a sibling temp file then renames into place.
// Rename is atomic on POSIX, so concurrent readers never observe a half-written file.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
