package service

import (
	"testing"

	"github.com/google/uuid"
)

func TestFilterOutBrowseItemsByIDCountsStackMatchedAssets(t *testing.T) {
	stackItem := BrowseItem{
		Type: "stack",
		ID:   "stack:11111111-1111-1111-1111-111111111111",
		Stack: &BrowseStack{
			MatchedMemberIDs: []uuid.UUID{
				uuid.New(),
				uuid.New(),
				uuid.New(),
			},
		},
	}
	assetItem := BrowseItem{
		Type: "asset",
		ID:   "asset:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	filtered, removedVisible, removedAssets := filterOutBrowseItemsByID(
		[]BrowseItem{stackItem, assetItem},
		[]BrowseItem{{ID: stackItem.ID}},
	)

	if removedVisible != 1 {
		t.Fatalf("expected 1 removed visible item, got %d", removedVisible)
	}
	if removedAssets != 3 {
		t.Fatalf("expected 3 removed matched assets, got %d", removedAssets)
	}
	if len(filtered) != 1 || filtered[0].ID != assetItem.ID {
		t.Fatalf("expected only asset item to remain, got %#v", filtered)
	}
}

func TestFilterOutBrowseItemsByIDCountsAssetDuplicatesAsOne(t *testing.T) {
	assetItem := BrowseItem{
		Type: "asset",
		ID:   "asset:aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
	}

	filtered, removedVisible, removedAssets := filterOutBrowseItemsByID(
		[]BrowseItem{assetItem},
		[]BrowseItem{{ID: assetItem.ID}},
	)

	if removedVisible != 1 {
		t.Fatalf("expected 1 removed visible item, got %d", removedVisible)
	}
	if removedAssets != 1 {
		t.Fatalf("expected 1 removed matched asset, got %d", removedAssets)
	}
	if len(filtered) != 0 {
		t.Fatalf("expected all items filtered, got %#v", filtered)
	}
}

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
