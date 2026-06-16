package inject

import (
	"context"
	"testing"

	"server/internal/agent/ref"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestFormatInstructionExtras(t *testing.T) {
	extras := FormatInstructionExtras(
		[]string{"r1_selected — 选中的 3 张"},
		[]string{"person: Alice (person_id=7)"},
	)
	require.Contains(t, extras, "Attached context:")
	require.Contains(t, extras, "r1_selected")
	require.Contains(t, extras, "Bound entities:")
	require.Contains(t, extras, "person_id=7")
}

func TestMaterializeContext_EmptyAssetIDs(t *testing.T) {
	store := ref.NewMemoryStore(0, 0)
	deps := Dependencies{
		RefStore: store,
		UserID:   1,
		ThreadID: "t1",
	}
	line, err := materializeContext(context.Background(), deps, ref.Scope{UserID: 1, ThreadID: "t1"}, ContextItem{
		Type: "selection", AssetIDs: nil, Label: "empty",
	})
	require.NoError(t, err)
	require.Empty(t, line)
	require.Empty(t, store.List(ref.Scope{UserID: 1, ThreadID: "t1"}))
}

func TestMaterializeContext_ScopeBinding(t *testing.T) {
	store := ref.NewMemoryStore(0, 0)
	id := uuid.New()
	store.Create(
		ref.Scope{UserID: 1, ThreadID: "t1"},
		ref.Plan{Op: "context.selection"},
		"selected",
		"3 assets",
		[]uuid.UUID{id},
		false,
	)

	_, refErr := store.Get(ref.Scope{UserID: 2, ThreadID: "t1"}, "r1_selected")
	require.NotNil(t, refErr)
	require.Equal(t, ref.CodeRefNotFound, refErr.Code)

	_, refErr = store.Get(ref.Scope{UserID: 1, ThreadID: "other"}, "r1_selected")
	require.NotNil(t, refErr)
}

func TestMaterializeContext_QuotaEviction(t *testing.T) {
	store := ref.NewMemoryStore(0, 2)
	scope := ref.Scope{UserID: 1, ThreadID: "t1"}

	for i := 0; i < 3; i++ {
		store.Create(scope, ref.Plan{Op: "context.selection"}, "sel", "1 asset", []uuid.UUID{uuid.New()}, false)
	}

	ledger := store.List(scope)
	require.Len(t, ledger, 2, "LRU eviction should cap refs at maxPerScope")
}
