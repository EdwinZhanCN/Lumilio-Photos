package processing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ItemState selects which items of a stage to list.
type ItemState string

const (
	ItemsFailed ItemState = "failed"
	ItemsQueued ItemState = "queued"
)

// MaxItemsLimit bounds one items page.
const MaxItemsLimit = 50

// Reason codes are the only failure detail in the public payload. Terminal
// errors that are not already one of these codes stay private.
const (
	ReasonUnsupportedMedia = "unsupported_media"
	ReasonRetriesExhausted = "processing_retry_exhausted"
	ReasonProcessingFailed = "processing_failed"
)

var ErrUnknownStage = errors.New("unknown processing stage")

// Item is one subject of a stage: a file, a Repository, or a catalog update.
type Item struct {
	SubjectID  string    `json:"subject_id"`
	AssetID    *string   `json:"asset_id,omitempty"`
	Label      string    `json:"label"`
	ReasonCode *string   `json:"reason_code,omitempty" enums:"unsupported_media,processing_retry_exhausted,processing_failed"`
	Attempts   *int64    `json:"attempts,omitempty"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ItemsPage is one bounded page. NextCursor is empty on the last page.
type ItemsPage struct {
	Items      []Item `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

func reasonCode(terminal sql.NullString) *string {
	if !terminal.Valid {
		return nil
	}
	code := ReasonProcessingFailed
	switch terminal.String {
	case ReasonUnsupportedMedia, ReasonRetriesExhausted:
		code = terminal.String
	}
	return &code
}

// cursor is (updated_at micros, subject id); failed pages run newest first,
// queued pages oldest first, matching what each list is for.
func parseCursor(raw string) (int64, string, bool) {
	at, subject, ok := strings.Cut(raw, ":")
	if !ok {
		return 0, "", false
	}
	value, err := strconv.ParseInt(at, 10, 64)
	if err != nil {
		return 0, "", false
	}
	return value, subject, true
}

// Items lists one page of a stage's failed or queued subjects.
func (r *Reader) Items(ctx context.Context, id StageID, state ItemState, limit int, cursor string) (ItemsPage, error) {
	spec, ok := LookupStage(id)
	if !ok {
		return ItemsPage{}, ErrUnknownStage
	}
	if state != ItemsFailed && state != ItemsQueued {
		return ItemsPage{}, fmt.Errorf("invalid item state %q", state)
	}
	if limit <= 0 || limit > MaxItemsLimit {
		limit = MaxItemsLimit
	}
	failed := state == ItemsFailed
	now := r.now().UTC()

	// Each source yields (subject, asset, label, terminal, attempts, updated).
	var source string
	var args []any
	switch {
	case spec.AssetStage != "":
		source = `SELECT s.asset_id AS subject, s.asset_id AS asset, a.original_filename AS label, s.terminal_error AS terminal, f.failure_count AS attempts, s.updated_at AS updated
		 FROM asset_pipeline_state s JOIN assets a ON a.asset_id=s.asset_id AND a.is_deleted=0
		 LEFT JOIN asset_pipeline_failures f ON f.asset_id=s.asset_id AND f.stage=s.stage
		  AND f.source_content_id=s.source_content_id AND f.pipeline_version=s.pipeline_version AND f.desired_version=s.desired_version
		 WHERE s.stage=? AND ` + pick(failed, `s.terminal_error IS NOT NULL`, `s.desired_version>s.applied_version AND s.terminal_error IS NULL`)
		args = append(args, string(spec.AssetStage))
	case id == StageImport:
		source = `SELECT receipt_id AS subject, NULL AS asset, subject_id AS label, terminal_error AS terminal, NULL AS attempts, updated_at AS updated FROM catalog_operation_receipts
		 WHERE kind='ingest' AND ` + pick(failed, `state='failed' AND updated_at>?`, `state='pending'`)
		if failed {
			args = append(args, now.Add(-ImportFailureWindow).UnixMicro())
		}
	case id == StageScan:
		source = `SELECT o.repository_id AS subject, NULL AS asset, COALESCE(r.name, o.repository_id) AS label, o.terminal_error AS terminal, NULL AS attempts, o.updated_at AS updated
		 FROM repository_observation_state o LEFT JOIN repositories r ON r.repo_id=o.repository_id
		 WHERE ` + pick(failed, `o.terminal_error IS NOT NULL`, `o.desired_epoch>o.applied_epoch AND o.terminal_error IS NULL`)
	case id == StageEvents:
		source = `SELECT CAST(e.owner_id AS TEXT) AS subject, NULL AS asset, COALESCE(u.username, CAST(e.owner_id AS TEXT)) AS label, e.terminal_error AS terminal, NULL AS attempts, e.updated_at AS updated
		 FROM event_projection_pipeline_state e LEFT JOIN users u ON u.user_id=e.owner_id
		 WHERE ` + pick(failed, `e.terminal_error IS NOT NULL`, `e.source_revision>e.applied_revision AND e.terminal_error IS NULL`)
	case id == StagePlaces:
		source = `SELECT l.repository_id||':'||l.owner_id AS subject, NULL AS asset, COALESCE(r.name, l.repository_id) AS label, l.terminal_error AS terminal, NULL AS attempts, l.updated_at AS updated
		 FROM location_projection_state l LEFT JOIN repositories r ON r.repo_id=l.repository_id
		 WHERE ` + pick(failed, `l.terminal_error IS NOT NULL`, `l.source_revision>l.published_revision AND l.terminal_error IS NULL`) + `
		 UNION ALL SELECT 'resolution', NULL, 'resolution', terminal_error, NULL, updated_at FROM location_resolution_pipeline_state
		 WHERE ` + pick(failed, `terminal_error IS NOT NULL`, `projection_version>applied_revision AND terminal_error IS NULL`)
	case id == StageTextSearch:
		source = `SELECT scope AS subject, NULL AS asset, scope AS label, terminal_error AS terminal, NULL AS attempts, updated_at AS updated FROM ocr_projection_pipeline_state
		 WHERE ` + pick(failed, `terminal_error IS NOT NULL`, `projection_version>applied_revision AND terminal_error IS NULL`)
	case id == StageBackup:
		source = `SELECT receipt_id AS subject, NULL AS asset, subject_id AS label, terminal_error AS terminal, NULL AS attempts, updated_at AS updated FROM (
		  SELECT *, ROW_NUMBER() OVER(PARTITION BY subject_id ORDER BY created_at DESC, receipt_id DESC) AS position
		  FROM catalog_operation_receipts WHERE kind='backup') WHERE position=1 AND ` + pick(failed, `state='failed'`, `state='pending'`)
	default:
		return ItemsPage{}, ErrUnknownStage
	}

	query := `SELECT subject, asset, label, terminal, attempts, updated FROM (` + source + `)`
	if at, subject, ok := parseCursor(cursor); ok {
		if failed {
			query += ` WHERE (updated < ? OR (updated = ? AND subject < ?))`
		} else {
			query += ` WHERE (updated > ? OR (updated = ? AND subject > ?))`
		}
		args = append(args, at, at, subject)
	}
	query += pick(failed, ` ORDER BY updated DESC, subject DESC`, ` ORDER BY updated ASC, subject ASC`) + ` LIMIT ?`
	args = append(args, limit+1)

	rows, err := r.catalog.QueryContext(ctx, query, args...)
	if err != nil {
		return ItemsPage{}, fmt.Errorf("read %s items: %w", id, err)
	}
	defer rows.Close()
	page := ItemsPage{Items: make([]Item, 0, limit)}
	for rows.Next() {
		var subject, label string
		var asset, terminal sql.NullString
		var attempts sql.NullInt64
		var updated int64
		if err := rows.Scan(&subject, &asset, &label, &terminal, &attempts, &updated); err != nil {
			return ItemsPage{}, err
		}
		if len(page.Items) == limit {
			last := page.Items[len(page.Items)-1]
			page.NextCursor = strconv.FormatInt(last.UpdatedAt.UnixMicro(), 10) + ":" + last.SubjectID
			break
		}
		item := Item{SubjectID: subject, Label: label, ReasonCode: reasonCode(terminal), UpdatedAt: time.UnixMicro(updated).UTC()}
		if asset.Valid {
			value := asset.String
			item.AssetID = &value
		}
		if attempts.Valid {
			value := attempts.Int64
			item.Attempts = &value
		}
		page.Items = append(page.Items, item)
	}
	return page, rows.Err()
}

func pick(condition bool, whenTrue, whenFalse string) string {
	if condition {
		return whenTrue
	}
	return whenFalse
}
