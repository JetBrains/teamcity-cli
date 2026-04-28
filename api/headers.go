package api

import (
	"net/http"
	"os"
	"strings"
)

// EnvHeaderPrefix is the env-var prefix that contributes extra HTTP headers to every request.
// TEAMCITY_HEADER_FOO_BAR=value sends "Foo-Bar: value" — underscores become hyphens, name is canonical-cased.
const EnvHeaderPrefix = "TEAMCITY_HEADER_"

// EnvHeaders returns extra headers gathered from TEAMCITY_HEADER_* env vars.
// Values containing CR/LF/NUL bytes are dropped at the boundary. Empty names and empty values are skipped.
// Returns nil if no matching env vars are set.
func EnvHeaders() map[string]string {
	var headers map[string]string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, EnvHeaderPrefix) {
			continue
		}
		eq := strings.IndexByte(e, '=')
		if eq < 0 {
			continue
		}
		rawName := e[len(EnvHeaderPrefix):eq]
		if rawName == "" {
			continue
		}
		value := e[eq+1:]
		if value == "" || strings.ContainsAny(value, "\r\n\x00") {
			continue
		}
		if headers == nil {
			headers = map[string]string{}
		}
		name := http.CanonicalHeaderKey(strings.ReplaceAll(rawName, "_", "-"))
		headers[name] = value
	}
	return headers
}
