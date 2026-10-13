// Package apispec embeds the OpenAPI specification (api/openapi.yaml) for the Help Center API reference.
package apispec

import _ "embed"

// OpenAPI is the raw YAML specification.
//
//go:embed openapi.yaml
var OpenAPI []byte
