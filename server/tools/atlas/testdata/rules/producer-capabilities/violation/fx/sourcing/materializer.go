package sourcing

import (
	"database/sql"

	"fx/repo"
)

// Materializer holds broad capabilities.
type Materializer struct {
	db      *sql.DB
	queries []*repo.Queries
}

// New takes the query set.
func New(q *repo.Queries) *Materializer { return &Materializer{queries: []*repo.Queries{q}} }

// Run builds a callback over the database.
func Run() any { return func(_ *sql.DB) {} }
