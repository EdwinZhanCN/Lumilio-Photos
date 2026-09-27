// Package migrations embeds the SQL catalog baseline and its forward steps so
// they can be applied without depending on the working directory or on the
// files being present on disk. This is required for the desktop bundle (which
// has no repo checkout and an unpredictable CWD) and also makes docker/dev
// startup CWD-independent.
package migrations

import "embed"

// FS holds the catalog baseline (schema version 1, frozen at the rc.1 tag)
// and steps/, the numbered forward steps that reach later versions. There are
// no down migrations. steps/README.md states the contributor rules.
//
//go:embed *.sql steps
var FS embed.FS
