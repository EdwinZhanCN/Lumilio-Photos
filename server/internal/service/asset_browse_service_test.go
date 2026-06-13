package service

import (
	"testing"

	"github.com/google/uuid"
)

func TestBrowseQueryResultFromItemsPaginatesVisibleItemsSeparatelyFromRawAssets(t *testing.T) {
	stackItem := BrowseItem{
		Type: "stack",
		ID:   "stack:11111111-1111-1111-1111-111111111111",
		Stack: &BrowseStack{
			MatchedMemberIDs: []uuid.UUID{
				uuid.New(),
				uuid.New(),
			},
		},
	}
	assetItem := BrowseItem{
		Type: "asset",
		ID:   "asset:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}
	items := []BrowseItem{stackItem, assetItem}

	firstPage := browseQueryResultFromItems(items, 3, StackModeCollapsed, 1, 0)
	if firstPage.TotalVisible != 2 {
		t.Fatalf("expected 2 visible items, got %d", firstPage.TotalVisible)
	}
	if firstPage.TotalAssets != 3 {
		t.Fatalf("expected 3 raw matched assets, got %d", firstPage.TotalAssets)
	}
	if len(firstPage.Items) != 1 || firstPage.Items[0].ID != stackItem.ID {
		t.Fatalf("expected first visible page to contain stack item, got %#v", firstPage.Items)
	}

	secondPage := browseQueryResultFromItems(items, 3, StackModeCollapsed, 10, 1)
	if len(secondPage.Items) != 1 || secondPage.Items[0].ID != assetItem.ID {
		t.Fatalf("expected second visible page to contain asset item, got %#v", secondPage.Items)
	}
}

func TestPreferredStackFocusAssetIDPrefersMatchedMemberOverCover(t *testing.T) {
	matchedID := uuid.New()
	coverID := uuid.New()

	stack := &BrowseStack{
		CoverAssetID:     coverID,
		MemberAssetIDs:   []uuid.UUID{coverID, matchedID},
		MatchedMemberIDs: []uuid.UUID{matchedID},
	}

	if got := preferredStackFocusAssetID(stack); got != matchedID {
		t.Fatalf("expected matched member focus, got %s", got)
	}
}

func TestPreferredStackFocusAssetIDFallsBackToCover(t *testing.T) {
	coverID := uuid.New()

	stack := &BrowseStack{
		CoverAssetID:   coverID,
		MemberAssetIDs: []uuid.UUID{coverID, uuid.New()},
	}

	if got := preferredStackFocusAssetID(stack); got != coverID {
		t.Fatalf("expected cover fallback, got %s", got)
	}
}
