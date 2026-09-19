// Package migrations embeds the SQL catalog baseline so it can be applied
// without depending on the working directory or on the files being present on
// disk. This is required for the desktop bundle (which has no repo checkout and
// an unpredictable CWD) and also makes docker/dev startup CWD-independent.
package migrations

import "embed"

// FS holds the single standalone catalog baseline. The schema is edited in
// place: there is no migration sequence, no down migration, and no historical
// generation to carry forward.
//
//go:embed *.sql
var FS embed.FS
