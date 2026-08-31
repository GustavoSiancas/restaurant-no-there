package backend

import "embed"

// Assets makes migrations and the API contract independent from the process working directory.
//
//go:embed migrations/*.up.sql docs/openapi.yaml
var Assets embed.FS
