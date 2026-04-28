package api

import (
	"net/http"
	"os"
	"strings"
)

// EnvHeaderPrefix is the env-var prefix that contributes extra HTTP headers to every request.
// TEAMCITY_HEADER_FOO_BAR=value sends "Foo-Bar: value" — underscores become hyphens, name is canonical-cased.
const EnvHeaderPrefix = "TEAMCITY_HEADER_"

// EnvHeaders returns TEAMCITY_HEADER_* env vars as a canonical-cased header map; constructors call this implicitly.
func EnvHeaders() map[string]string {
	var headers map[string]string
	for _, e := range os.Environ() {
		key, value, ok := strings.Cut(e, "=")
		if !ok || !strings.HasPrefix(key, EnvHeaderPrefix) {
			continue
		}
		suffix := key[len(EnvHeaderPrefix):]
		if suffix == "" || value == "" || strings.ContainsAny(value, "\r\n\x00") {
			continue
		}
		if headers == nil {
			headers = map[string]string{}
		}
		headers[http.CanonicalHeaderKey(strings.ReplaceAll(suffix, "_", "-"))] = value
	}
	return headers
}
