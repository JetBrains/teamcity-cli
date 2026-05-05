package analytics

import (
	"os"
	"path/filepath"
	"time"
)

// CDN metadata is TTL-cached positively by fus, but not negatively. When the
// metadata endpoint returns an error (e.g. 403 before AP registration), the fus
// cache stays empty and every CLI invocation re-attempts the round-trip. The
// marker below holds a negative result so we skip the round-trip for cdnBackoff
// after any failure. CDN is still preferred — the next attempt after the
// backoff will promote a successful fetch into fus's positive TTL cache.
const (
	cdnUnavailableMarker = "cdn-unavailable"
	cdnBackoff           = time.Hour
)

func cdnMarkerPath(dir string) string {
	return filepath.Join(dir, cdnUnavailableMarker)
}

func shouldTryCDN(dir string) bool {
	info, err := os.Stat(cdnMarkerPath(dir))
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > cdnBackoff
}

func markCDNUnavailable(dir string) {
	_ = os.WriteFile(cdnMarkerPath(dir), nil, 0o600)
}

func clearCDNUnavailable(dir string) {
	_ = os.Remove(cdnMarkerPath(dir))
}
