package catalog

import (
	"errors"

	"github.com/mattn/go-sqlite3"
)

// IsUniqueViolation reports whether err is SQLite rejecting a row because it
// repeats a UNIQUE or PRIMARY KEY value, so callers above the catalog can
// tell a lost race from a failure without importing the engine driver.
func IsUniqueViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
		sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
}
