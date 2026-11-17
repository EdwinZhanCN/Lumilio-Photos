package jobs

import "testing"

func TestRuntimeCatalogIsClosedMacroCatalog(t *testing.T) {
	want := map[string]bool{"ingest_asset": true, "analyze_asset": true, "generate_asset_derivatives": true, "transcode_media": true, "enrich_asset": true, "scan_repository_batch": true, "rebuild_projection_batch": true, "backup_catalog": true}
	got := map[string]bool{}
	for _, job := range RuntimeJobCatalog() {
		if job.InsertOpts().Queue != QueueMacro {
			t.Fatalf("%s queue=%q", job.Kind(), job.InsertOpts().Queue)
		}
		got[job.Kind()] = true
	}
	if len(got) != len(want) {
		t.Fatalf("catalog=%v", got)
	}
	for kind := range want {
		if !got[kind] {
			t.Fatalf("missing %s", kind)
		}
	}
}
