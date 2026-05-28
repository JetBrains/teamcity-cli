// Package schemas embeds the JSON Schemas published under schemas/ so the CLI and external consumers share one source of truth.
package schemas

import _ "embed"

//go:embed pipeline.json
var Pipeline []byte

//go:embed teamcity.toml.json
var TeamcityTOML []byte
