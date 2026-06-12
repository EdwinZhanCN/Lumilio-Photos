// Package ref implements the server-side handle store for the agent ref
// system. Agent tools exchange ordered asset-ID snapshots through short
// ref ids instead of inlining asset data into the LLM context; the frontend
// hydrates refs over HTTP, so asset data never crosses the model boundary.
// See docs/agent/exec-plans/active/agent-ref-system.md for the contracts
// and invariants this package enforces.
package ref

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultTTL is how long a ref survives without being referenced.
	DefaultTTL = 2 * time.Hour
	// DefaultMaxRefsPerScope caps active refs per (user, thread); the least
	// recently accessed ref is evicted when the cap is exceeded.
	DefaultMaxRefsPerScope = 64
	// MaxSnapshotSize caps the number of asset IDs materialized into one ref.
	// Producers must truncate and surface Truncated in the receipt summary.
	MaxSnapshotSize = 10000
	// maxHintLen bounds the mnemonic suffix of a ref id.
	maxHintLen = 12
)

// Scope binds a ref to its owner conversation (INV-4). Refs are never
// visible across users or threads.
type Scope struct {
	UserID   int32
	ThreadID string
}

// Plan records the expression that produced a ref: operator, parameters and
// parent refs. It is lineage/provenance only — never replayed today — and the
// escape hatch for a future lazy RefStore.
type Plan struct {
	Op      string            `json:"op"`
	Params  map[string]string `json:"params,omitempty"`
	Parents []string          `json:"parents,omitempty"`
}

// Ref is an immutable, ordered snapshot of asset IDs (INV-5). Order is
// semantic: producers write relevance/recency order, transformers rewrite it.
// Callers must treat AssetIDs as read-only; Slice is the paging accessor.
type Ref struct {
	ID         string
	Scope      Scope
	Plan       Plan
	AssetIDs   []uuid.UUID
	Truncated  bool
	CreatedAt  time.Time
	LastAccess time.Time
	// Summary is the receipt one-liner; it feeds the ref ledger injected
	// into the agent instruction each turn.
	Summary string

	seq int // per-scope creation order, used by List
}

// Count returns the number of assets in the snapshot.
func (r *Ref) Count() int { return len(r.AssetIDs) }

// Slice returns the page [offset, offset+limit) of the snapshot, preserving
// order. Out-of-range pages return an empty slice.
func (r *Ref) Slice(offset, limit int) []uuid.UUID {
	if offset < 0 || limit <= 0 || offset >= len(r.AssetIDs) {
		return nil
	}
	end := offset + limit
	if end > len(r.AssetIDs) {
		end = len(r.AssetIDs)
	}
	return r.AssetIDs[offset:end]
}

// ToolReceipt is the only shape a ref-producing tool may return to the LLM
// (INV-1, INV-3): the handle, the cardinality and a one-line summary.
// A count of zero must be stated plainly in Summary.
type ToolReceipt struct {
	RefID   string `json:"ref_id"`
	Count   int    `json:"count"`
	Summary string `json:"summary"`
}

func formatID(seq int, hint string) string {
	hint = sanitizeHint(hint)
	if hint == "" {
		return fmt.Sprintf("r%d", seq)
	}
	return fmt.Sprintf("r%d_%s", seq, hint)
}

// sanitizeHint reduces a mnemonic to lowercase [a-z0-9_], collapsing other
// runes to single underscores and truncating to maxHintLen. The hint is a
// mnemonic only — uniqueness comes from the per-scope sequence number, and a
// hint must not assert anything the ref's metadata cannot back.
func sanitizeHint(hint string) string {
	hint = strings.TrimSpace(strings.ToLower(hint))
	var b strings.Builder
	lastUnderscore := true // suppress leading underscore
	for _, r := range hint {
		if b.Len() >= maxHintLen {
			break
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}
