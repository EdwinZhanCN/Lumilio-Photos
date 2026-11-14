package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// split-report resolves uses through the types: file-to-file edges carry the
// declarations used, a cut lists the edges between clusters and the
// unexported symbols it would have to export, and a cycle between clusters
// is reported because it blocks the cut.
func TestSplitReportFindsCrossingSymbolsAndCycles(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "splitreport"))
	if err != nil {
		t.Fatal(err)
	}
	config := &Config{GoModules: []GoModule{{Dir: "fx", Path: "fx", Side: "server"}}}
	plain, err := runSplitReport(root, config, "fx/cut", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"4 files",
		"- assets.go → people.go: Name, Person, Person.Greeting",
		"- people.go → assets.go: formatID",
		"- music.go → assets.go: Asset",
		"- assets.go, people.go\n",
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("report without clusters lacks %q:\n%s", want, plain)
		}
	}

	clusters := filepath.Join(t.TempDir(), "clusters.yaml")
	if err := os.WriteFile(clusters, []byte("clusters:\n  assets: [assets.go]\n  people: [people.go]\n  music: [music.go]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cut, err := runSplitReport(root, config, "cut", clusters)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| (remainder) | doc.go |",
		"- music → assets: Asset\n",
		"- people → assets: formatID\n",
		"## Cycles\n\n- assets, people\n",
	} {
		if !strings.Contains(cut, want) {
			t.Errorf("report with clusters lacks %q:\n%s", want, cut)
		}
	}
	unexported := cut[strings.Index(cut, "## Unexported"):strings.Index(cut, "## Cycles")]
	if strings.Contains(unexported, "Asset") || strings.Contains(unexported, "Person") {
		t.Errorf("only unexported symbols cross a cut:\n%s", unexported)
	}

	if err := os.WriteFile(clusters, []byte("clusters:\n  a: [assets.go]\n  b: [assets.go]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runSplitReport(root, config, "cut", clusters); err == nil || !strings.Contains(err.Error(), "is in clusters") {
		t.Errorf("a file in two clusters must be refused, got %v", err)
	}
}
