package catalog

import (
	"database/sql"

	"github.com/riverqueue/river/riverdriver"
	"github.com/riverqueue/river/riverdriver/riversqlite"
)

// RiverDriver returns River's SQLite driver over this QueueDB. Every River
// mutation and transaction stays on the QueueDB's sole writer, while the
// notification listener polls the query-only reader pool: riversqlite
// otherwise uses one *sql.DB for its executor and its 50ms
// NotificationGetAfter SELECT loop, needlessly admitting read polling through
// the sole writer connection.
//
// Embedding the pinned upstream driver deliberately inherits its unstable
// interface; the compile-time assertion and held-writer test make a River
// upgrade fail locally if that delegation boundary changes.
func (q *QueueDB) RiverDriver() riverdriver.Driver[*sql.Tx] {
	return &riverSplitDriver{
		Driver:   riversqlite.New(q.SQL),
		listener: riversqlite.New(q.ReaderSQL),
	}
}

type riverSplitDriver struct {
	*riversqlite.Driver
	listener *riversqlite.Driver
}

var _ riverdriver.Driver[*sql.Tx] = (*riverSplitDriver)(nil)

func (d *riverSplitDriver) GetListener(params *riverdriver.GetListenenerParams) riverdriver.Listener {
	return d.listener.GetListener(params)
}
