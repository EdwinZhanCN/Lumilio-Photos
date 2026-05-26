package service

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestINaturalistSpeciesReferenceFetchesWikiAndReferenceImage(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/taxa":
			if got := r.URL.Query().Get("q"); got != "Rucervus duvaucelii" {
				t.Fatalf("unexpected search query: %s", got)
			}
			if got := r.URL.Query().Get("locale"); got != "zh" {
				t.Fatalf("unexpected search locale: %s", got)
			}
			return jsonResponse(`{"results":[{"id":75046}]}`), nil
		case "/taxa/75046":
			if got := r.URL.Query().Get("locale"); got != "zh" {
				t.Fatalf("unexpected detail locale: %s", got)
			}
			return jsonResponse(`{
				"results":[{
					"id":75046,
					"name":"Rucervus duvaucelii",
					"preferred_common_name":"Barasingha",
					"wikipedia_summary":"The <b>barasingha</b> (<i>Rucervus duvaucelii</i>) is a deer species.",
					"wikipedia_url":"https://en.wikipedia.org/wiki/Rucervus_duvaucelii",
					"default_photo":{
						"id":231650420,
						"license_code":"cc-by",
						"attribution":"(c) Ramesh Shenai Jr., some rights reserved (CC BY)",
						"medium_url":"https://example.test/medium.jpeg",
						"large_url":"https://example.test/large.jpeg"
					}
				}]
			}`), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(`{}`), nil
	})}

	service := newINaturalistSpeciesReferenceService("https://inat.test", client)
	ref, err := service.FetchReference(t.Context(), SpeciesReferenceQuery{
		ScientificName: "Rucervus duvaucelii",
		CommonName:     "Barasingha",
		Locale:         "zh",
	})
	if err != nil {
		t.Fatalf("FetchReference returned error: %v", err)
	}

	if ref.Provider != speciesReferenceProviderINaturalist {
		t.Fatalf("unexpected provider: %s", ref.Provider)
	}
	if ref.ScientificName != "Rucervus duvaucelii" || ref.CommonName != "Barasingha" {
		t.Fatalf("unexpected names: %#v", ref)
	}
	if ref.WikipediaSummary != "The barasingha (Rucervus duvaucelii) is a deer species." {
		t.Fatalf("unexpected summary: %s", ref.WikipediaSummary)
	}
	if ref.ImageURL != "https://example.test/large.jpeg" {
		t.Fatalf("unexpected image url: %s", ref.ImageURL)
	}
	if ref.ImageSourceURL != "https://www.inaturalist.org/photos/231650420" {
		t.Fatalf("unexpected source url: %s", ref.ImageSourceURL)
	}
}

func TestINaturalistSpeciesReferenceFallsBackToCommonName(t *testing.T) {
	searches := 0
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/taxa":
			searches++
			if r.URL.Query().Get("q") == "Missing scientific name" {
				return jsonResponse(`{"results":[]}`), nil
			}
			return jsonResponse(`{"results":[{"id":42}]}`), nil
		case "/taxa/42":
			return jsonResponse(`{"results":[{"id":42,"name":"Sciurus carolinensis","preferred_common_name":"Eastern Gray Squirrel"}]}`), nil
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		return jsonResponse(`{}`), nil
	})}

	service := newINaturalistSpeciesReferenceService("https://inat.test", client)
	ref, err := service.FetchReference(t.Context(), SpeciesReferenceQuery{
		ScientificName: "Missing scientific name",
		CommonName:     "Eastern Gray Squirrel",
	})
	if err != nil {
		t.Fatalf("FetchReference returned error: %v", err)
	}
	if searches != 2 {
		t.Fatalf("expected two searches, got %d", searches)
	}
	if ref.TaxonID != 42 {
		t.Fatalf("unexpected taxon id: %d", ref.TaxonID)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
