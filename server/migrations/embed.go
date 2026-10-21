package migrations

import "embed"

// FS holds the catalog baseline (schema version 1, frozen at the rc.1 tag)
// and steps/, the numbered forward steps that reach later versions. There are
// no down migrations. steps/README.md states the contributor rules.
//
//go:embed *.sql steps
var FS embed.FS
