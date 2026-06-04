// Package migrations embeds the SQL migration files so they can be applied
// without depending on the working directory or on the files being present on
// disk. This is required for the desktop bundle (which has no repo checkout and
// an unpredictable CWD) and also makes docker/dev migrations CWD-independent.
package migrations

import "embed"

// FS holds the up/down migration files (NNNN_name.up.sql / .down.sql), consumed
// via golang-migrate's iofs source driver.
//
//go:embed *.sql
var FS embed.FS
