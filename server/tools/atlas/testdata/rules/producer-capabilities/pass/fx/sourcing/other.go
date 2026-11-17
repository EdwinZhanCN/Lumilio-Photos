package sourcing

import "database/sql"

// Store is outside the rule's files and may hold the database.
type Store struct{ db *sql.DB }
