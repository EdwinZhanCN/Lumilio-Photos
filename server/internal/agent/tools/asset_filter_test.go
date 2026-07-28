package tools

import (
	"strings"
	"testing"
)

// The agent filter speaks the same media-item vocabulary as browse: a
// composition value, a stack membership, and stack kinds. There is no RAW
// boolean, and unstacked+kinds is rejected exactly as the HTTP API rejects it.

func TestBuildFilterParamsAcceptsCompositionAndStackFilters(t *testing.T) {
	params, err := buildFilterParams(&AssetFilterInput{
		Composition:     "JPEG_RAW",
		StackMembership: "Stacked",
		StackKinds:      []string{"Burst", " manual "},
	})
	if err != nil {
		t.Fatalf("buildFilterParams: %v", err)
	}
	if params.Composition != "jpeg_raw" {
		t.Fatalf("composition = %v, want jpeg_raw", params.Composition)
	}
	if params.StackMembership != "stacked" {
		t.Fatalf("stack_membership = %v, want stacked", params.StackMembership)
	}
	if params.StackKinds == nil || !strings.Contains(*params.StackKinds, "burst") ||
		!strings.Contains(*params.StackKinds, "manual") {
		t.Fatalf("stack_kinds = %v, want both burst and manual", params.StackKinds)
	}
}

func TestBuildFilterParamsRejectsInvalidBrowseFilters(t *testing.T) {
	cases := map[string]*AssetFilterInput{
		"unknown composition":  {Composition: "sidecar"},
		"unknown membership":   {StackMembership: "grouped"},
		"unknown stack kind":   {StackKinds: []string{"panorama"}},
		"unstacked with kinds": {StackMembership: "unstacked", StackKinds: []string{"burst"}},
	}

	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := buildFilterParams(input); err == nil {
				t.Fatalf("buildFilterParams(%+v) accepted an invalid filter", input)
			}
		})
	}
}

func TestBuildFilterParamsAcceptsLivePhotoComposition(t *testing.T) {
	params, err := buildFilterParams(&AssetFilterInput{Composition: "Live_Photo"})
	if err != nil {
		t.Fatalf("buildFilterParams: %v", err)
	}
	if params.Composition != "live_photo" {
		t.Fatalf("composition = %v, want live_photo", params.Composition)
	}
}

func TestBuildFilterParamsAcceptsUnstackedWithoutKinds(t *testing.T) {
	params, err := buildFilterParams(&AssetFilterInput{StackMembership: "unstacked"})
	if err != nil {
		t.Fatalf("buildFilterParams: %v", err)
	}
	if params.StackMembership != "unstacked" {
		t.Fatalf("stack_membership = %v, want unstacked", params.StackMembership)
	}
	if params.StackKinds != nil {
		t.Fatalf("stack_kinds = %v, want nil", params.StackKinds)
	}
}

func TestFilterSummaryCountsMediaItems(t *testing.T) {
	summary := filterSummary(&AssetFilterInput{Composition: "jpeg_raw"}, 7, false)
	if !strings.Contains(summary, "7 media items") {
		t.Fatalf("summary = %q, want a media-item count", summary)
	}
	if strings.Contains(summary, "assets") {
		t.Fatalf("summary = %q, must not count physical assets", summary)
	}
}

func TestFilterPlanParamsCarryBrowseFilters(t *testing.T) {
	params := filterPlanParams(&AssetFilterInput{
		Composition:     "raw_unpaired",
		StackMembership: "stacked",
		StackKinds:      []string{"burst", "manual"},
	})

	for key, want := range map[string]string{
		"composition":      "raw_unpaired",
		"stack_membership": "stacked",
		"stack_kinds":      "burst,manual",
	} {
		if params[key] != want {
			t.Fatalf("plan param %q = %q, want %q", key, params[key], want)
		}
	}
	if _, present := params["raw"]; present {
		t.Fatalf("plan params still carry a retired raw filter: %v", params)
	}
}

func TestFilterHintPrefersCompositionAndStackKind(t *testing.T) {
	if got := filterHint(&AssetFilterInput{Composition: "jpeg_raw"}); got != "jpeg_raw" {
		t.Fatalf("hint = %q, want jpeg_raw", got)
	}
	if got := filterHint(&AssetFilterInput{StackKinds: []string{"burst"}}); got != "burst" {
		t.Fatalf("hint = %q, want burst", got)
	}
}
