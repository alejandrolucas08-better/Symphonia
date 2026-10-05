package migrations

import "embed"

// Files travel with the binary so startup does not depend on source paths.
//
//go:embed *.sql
var Files embed.FS
