// Package ref implements the server-side handle store for the agent ref
// system. Agent tools exchange ordered asset-ID snapshots through short
// ref ids instead of inlining asset data into the LLM context; the frontend
// hydrates refs over HTTP, so asset data never crosses the model boundary.
// See site/docs/internal/agent/exec-plans/active/agent-ref-system.md for the contracts
// and invariants this package enforces.
package ref

import (
	"encoding/json"
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

// Plan is the versioned expression envelope that produced a ref. Producer
// plans may be replayed only by a handler for the exact schema/tool version;
// transformed plans remain lineage-only because they depend on parent refs.
type Plan struct {
	SchemaVersion         int             `json:"schema_version"`
	ToolVersion           string          `json:"tool_version"`
	Op                    string          `json:"op"`
	Payload               json.RawMessage `json:"payload,omitempty"`
	EmbeddingIndexVersion string          `json:"embedding_index_version,omitempty"`
	AuthorizationScope    PlanAuthScope   `json:"authorization_scope"`
	CreationPolicyVersion int             `json:"creation_policy_version"`
	Parents               []string        `json:"parents,omitempty"`
}

type PlanAuthScope struct {
	UserID int32 `json:"user_id"`
}

const (
	CurrentPlanSchemaVersion = 1
	CurrentToolVersion       = "agent-tools/v1"
	CurrentPolicyVersion     = 1
)

func (p Plan) Normalize(userID int32) (Plan, error) {
	if p.SchemaVersion == 0 {
		if len(p.Payload) == 0 {
			p.Payload = json.RawMessage("{}")
		}
		p.SchemaVersion = CurrentPlanSchemaVersion
		p.ToolVersion = CurrentToolVersion
		p.AuthorizationScope = PlanAuthScope{UserID: userID}
		p.CreationPolicyVersion = CurrentPolicyVersion
	}
	if p.SchemaVersion != CurrentPlanSchemaVersion ||
		p.ToolVersion != CurrentToolVersion ||
		p.AuthorizationScope.UserID != userID ||
		p.CreationPolicyVersion != CurrentPolicyVersion {
		return Plan{}, fmt.Errorf("unsupported or unauthorized plan envelope")
	}
	return p, nil
}

// TypedPayload serializes a statically known plan payload. Agent tools only
// pass simple DTOs/maps here, so a marshal failure is a programmer error.
func TypedPayload(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("marshal typed Agent plan payload: %v", err))
	}
	return payload
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
