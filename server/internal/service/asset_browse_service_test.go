package service

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeStackModeCollapsesUnknownValues(t *testing.T) {
	for input, want := range map[string]string{
		"":            StackModeCollapsed,
		"collapsed":   StackModeCollapsed,
		"expanded":    StackModeExpanded,
		"  expanded ": StackModeExpanded,
		"grouped":     StackModeCollapsed,
	} {
		if got := normalizeStackMode(input); got != want {
			t.Fatalf("normalizeStackMode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestValidateBrowseFilterRejectsUnstackedWithKinds(t *testing.T) {
	err := validateBrowseFilter(QueryAssetsParams{
		StackMembership: StackMembershipUnstacked,
		StackKinds:      []string{"burst"},
	})
	if !errors.Is(err, ErrInvalidBrowseFilter) {
		t.Fatalf("expected ErrInvalidBrowseFilter, got %v", err)
	}
}

func TestValidateBrowseFilterAcceptsCompatibleStackFilters(t *testing.T) {
	cases := []QueryAssetsParams{
		{},
		{StackMembership: StackMembershipUnstacked},
		{StackMembership: StackMembershipStacked, StackKinds: []string{"burst", "manual"}},
		{StackKinds: []string{"manual"}},
		{MediaComposition: MediaCompositionJPEGRAW},
	}
	for _, params := range cases {
		if err := validateBrowseFilter(params); err != nil {
			t.Fatalf("validateBrowseFilter(%+v) = %v, want nil", params, err)
		}
	}
}

func TestBrowseRowIDsAreTypePrefixed(t *testing.T) {
	mediaItemID := uuid.New()
	stackID := uuid.New()

	if got, want := browseMediaItemRowID(mediaItemID), "media:"+mediaItemID.String(); got != want {
		t.Fatalf("browseMediaItemRowID = %q, want %q", got, want)
	}
	if got, want := browseStackRowID(stackID), "stack:"+stackID.String(); got != want {
		t.Fatalf("browseStackRowID = %q, want %q", got, want)
	}
}

func TestPageBrowseItemsPaginatesVisibleRows(t *testing.T) {
	stackItem := BrowseItem{Type: BrowseItemTypeStack, ID: "stack:a"}
	mediaItem := BrowseItem{Type: BrowseItemTypeMediaItem, ID: "media:b"}
	items := []BrowseItem{stackItem, mediaItem}

	first := pageBrowseItems(items, 1, 0)
	if len(first) != 1 || first[0].ID != stackItem.ID {
		t.Fatalf("first page = %#v, want the stack row", first)
	}

	second := pageBrowseItems(items, 10, 1)
	if len(second) != 1 || second[0].ID != mediaItem.ID {
		t.Fatalf("second page = %#v, want the media item row", second)
	}

	if got := pageBrowseItems(items, 10, 5); len(got) != 0 {
		t.Fatalf("offset past the end = %#v, want empty", got)
	}
	if got := pageBrowseItems(items, 0, 0); len(got) != 0 {
		t.Fatalf("zero limit = %#v, want empty", got)
	}
}
