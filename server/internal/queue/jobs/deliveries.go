package jobs

import (
	"context"
	"database/sql"
	"time"
)

// DeliveryGroup is the execution state of one macro kind (and projection kind
// for rebuild batches). It describes disposable delivery records only; it is
// never product state.
type DeliveryGroup struct {
	Kind           string
	ProjectionKind string
	Running        int64
	Retryable      int64
	LastActivityAt *time.Time
}

// ReadDeliveryGroups aggregates River deliveries for read-only monitoring.
func ReadDeliveryGroups(ctx context.Context, queue *sql.DB) ([]DeliveryGroup, error) {
	rows, err := queue.QueryContext(ctx, `
 SELECT kind, COALESCE(json_extract(args,'$.projectionKind'),''),
  COUNT(*) FILTER (WHERE state='running'),
  COUNT(*) FILTER (WHERE state='retryable'),
  CAST(unixepoch(MAX(COALESCE(finalized_at, attempted_at))) AS INTEGER)
 FROM river_job
 GROUP BY 1, 2`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groups []DeliveryGroup
	for rows.Next() {
		var group DeliveryGroup
		var last sql.NullInt64
		if err := rows.Scan(&group.Kind, &group.ProjectionKind, &group.Running, &group.Retryable, &last); err != nil {
			return nil, err
		}
		if last.Valid {
			at := time.Unix(last.Int64, 0).UTC()
			group.LastActivityAt = &at
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}
