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

	migration := strings.ReplaceAll(string(sql), "\r\n", "\n")
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

func TestBreakGlassSecurityMigrationAddsAuthenticationState(t *testing.T) {
	up, err := FS.ReadFile("000008_breakglass_security.up.sql")
	if err != nil {
		t.Fatalf("read break-glass migration: %v", err)
	}
	migration := strings.ToLower(string(up))
	for _, required := range []string{
		"auth_version bigint not null default 0",
		"password_change_required boolean not null default false",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("break-glass migration missing %q", required)
		}
	}
}

func TestAssetBaselineUsesLayeredHashes(t *testing.T) {
	sql, err := FS.ReadFile("000003_assets_repositories.up.sql")
	if err != nil {
		t.Fatalf("read asset baseline migration: %v", err)
	}
	migration := strings.ToLower(string(sql))
	for _, required := range []string{
		"content_hash character varying(64) not null",
		"quick_fingerprint character varying(64)",
		"quick_fingerprint_version character varying(32)",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("asset baseline migration missing layered hash contract %q", required)
		}
	}
}

func TestHostOwnerMigrationRepairsRepositoryAndDerivedOwnership(t *testing.T) {
	sql, err := FS.ReadFile("000010_host_owner.up.sql")
	if err != nil {
		t.Fatalf("read Host Owner migration: %v", err)
	}
	migration := strings.ToLower(string(sql))
	for _, required := range []string{
		"order by created_at asc, user_id asc",
		"update repositories",
		"update assets",
		"update media_items",
		"update asset_stacks",
		"update duplicate_groups",
		"update face_clusters",
		"update location_clusters",
		"insert into location_cluster_assets",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("Host Owner migration missing %q", required)
		}
	}
	if !strings.Contains(migration, "where owner_id is null") {
		t.Fatal("Host Owner migration must preserve explicitly owned assets")
	}
}

func TestCloudOwnershipMigrationPinsCredentialBindingAndRunOwners(t *testing.T) {
	sql, err := FS.ReadFile("000011_cloud_ownership.up.sql")
	if err != nil {
		t.Fatalf("read cloud ownership migration: %v", err)
	}
	migration := strings.ToLower(string(sql))
	for _, required := range []string{
		"rename column created_by_user_id to owner_id",
		"alter column owner_id set not null",
		"update repository_cloud_bindings",
		"update cloud_import_runs",
		"repository_cloud_bindings_owner_id_fkey",
		"cloud_import_runs_owner_id_fkey",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("cloud ownership migration missing %q", required)
		}
	}
}
