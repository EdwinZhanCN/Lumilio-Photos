package migrations

import (
	"strings"
	"testing"
)

func TestMLBaselineSeedsUtilityClassifiers(t *testing.T) {
	sql, err := FS.ReadFile("000005_ml_analysis_results.up.sql")
	if err != nil {
		t.Fatalf("read ML baseline migration: %v", err)
	}

	migration := string(sql)
	if !strings.Contains(migration, "INSERT INTO public.classifier_definitions") {
		t.Fatal("ML baseline migration does not seed classifier definitions")
	}
	for _, required := range []string{
		"a comic book page",
		"a manga page with text and speech bubbles",
		"a comic panel with dialogue",
		"an illustrated story page",
		"a screenshot of a digital comic",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("ML baseline migration missing illustration prompt %q", required)
		}
	}
	if count := strings.Count(migration, "    0.03\n"); count != 3 {
		t.Fatalf("ML baseline migration seeds %d classifiers at threshold 0.03, want 3", count)
	}

	for slug, classifier := range map[string]struct {
		displayName string
		tagName     string
	}{
		"documents":    {displayName: "Documents", tagName: "document"},
		"receipts":     {displayName: "Receipts", tagName: "receipt"},
		"illustration": {displayName: "Illustration", tagName: "illustration"},
	} {
		t.Run(slug, func(t *testing.T) {
			rowPrefix := "(\n    '" + slug + "',\n    '" + classifier.displayName + "',\n    '" + classifier.tagName + "',"
			if !strings.Contains(migration, rowPrefix) {
				t.Fatalf(
					"ML baseline migration does not seed classifier %q with tag %q",
					slug,
					classifier.tagName,
				)
			}
		})
	}
}

func TestCollectionsBaselineUsesLogicalMediaItems(t *testing.T) {
	sql, err := FS.ReadFile("000004_collections_locations_duplicates.up.sql")
	if err != nil {
		t.Fatalf("read collections baseline migration: %v", err)
	}
	migration := string(sql)
	for _, required := range []string{
		"CREATE TABLE public.media_items",
		"CREATE TABLE public.media_item_assets",
		"CREATE TABLE public.asset_stacks",
		"CREATE TABLE public.asset_stack_members",
		"media_item_id uuid NOT NULL",
		"stack_kind = ANY (ARRAY['manual'::text, 'burst'::text])",
		"CREATE TRIGGER trg_assets_create_media_item",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("collections baseline missing logical-media contract %q", required)
		}
	}
	if strings.Contains(migration, "'raw_jpeg'::text") {
		t.Fatal("RAW/JPEG must be a media-item structure, not a presentation stack kind")
	}
}
