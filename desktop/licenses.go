package main

import (
	"embed"
	"net/http"
)

// licenseTexts holds the application's license and generated third-party
// notices. Build scripts also stage these files for offline access.
//
//go:embed licenses/*.txt
var licenseTexts embed.FS

func serveLegalText(file string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, err := licenseTexts.ReadFile(file)
		if err != nil {
			http.Error(w, "legal text missing from build", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	}
}

func handleTermsOfUse(w http.ResponseWriter, r *http.Request) {
	file := "licenses/TERMS-OF-USE.en.txt"
	if r.URL.Query().Get("lang") == "zh" {
		file = "licenses/TERMS-OF-USE.zh-CN.txt"
	}
	serveLegalText(file)(w, r)
}
