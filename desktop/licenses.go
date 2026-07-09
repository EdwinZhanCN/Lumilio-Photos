package main

import (
	"embed"
	"net/http"
)

// licenseTexts holds the full license texts shown by the onboarding window and
// staged into the bundle by the build scripts (Resources/licenses). The files
// are canonical SPDX texts; see licenses/README.md for provenance.
//
//go:embed licenses/*.txt
var licenseTexts embed.FS

// licenseEntry is one bundled component whose license the user can read during
// onboarding.
type licenseEntry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	License string `json:"license"`
	file    string
}

// licenseManifest lists the app itself first, then every bundled native
// component. Keep it in sync with what the build scripts actually stage
// (desktop/scripts/build-*.sh, fetch-resources.*).
var licenseManifest = []licenseEntry{
	{ID: "lumilio", Name: "Lumilio Photos", License: "GNU GPL v3.0", file: "licenses/GPL-3.0.txt"},
	{ID: "postgresql", Name: "PostgreSQL & pgvector", License: "PostgreSQL License", file: "licenses/PostgreSQL.txt"},
	{ID: "ffmpeg", Name: "FFmpeg", License: "GNU GPL v2.0 (bundled static build)", file: "licenses/GPL-2.0.txt"},
	{ID: "libvips", Name: "libvips", License: "GNU LGPL v2.1", file: "licenses/LGPL-2.1.txt"},
	{ID: "exiftool", Name: "ExifTool", License: "Perl Artistic License", file: "licenses/Artistic-1.0-Perl.txt"},
	{ID: "wails", Name: "Wails", License: "MIT License", file: "licenses/MIT-Wails.txt"},
}

// handleLicenseIndex serves the component list the onboarding page renders.
func handleLicenseIndex(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, licenseManifest)
}

// handleLicenseText serves one full license text (?id=<manifest id>).
func handleLicenseText(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	for _, e := range licenseManifest {
		if e.ID != id {
			continue
		}
		data, err := licenseTexts.ReadFile(e.file)
		if err != nil {
			http.Error(w, "license text missing from build", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
		return
	}
	http.Error(w, "unknown license id", http.StatusNotFound)
}
