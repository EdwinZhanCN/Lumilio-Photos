package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	speciesReferenceProviderINaturalist = "inaturalist"
	defaultINaturalistAPIBaseURL        = "https://api.inaturalist.org/v1"
)

var (
	ErrSpeciesReferenceNotFound = errors.New("species reference not found")
	errSpeciesReferenceNoQuery  = errors.New("scientific_name or common_name is required")
	htmlTagPattern              = regexp.MustCompile(`<[^>]+>`)
	iNaturalistLocalePattern    = regexp.MustCompile(`^[A-Za-z]{2,3}([-_][A-Za-z0-9]{2,8})?$`)
)

type SpeciesReferenceQuery struct {
	ScientificName string
	CommonName     string
	Locale         string
}

type SpeciesReference struct {
	Provider         string
	Query            string
	TaxonID          int
	ScientificName   string
	CommonName       string
	WikipediaSummary string
	WikipediaURL     string
	ReferenceURL     string
	ImageURL         string
	ImageAttribution string
	ImageLicense     string
	ImageSourceURL   string
}

type SpeciesReferenceService interface {
	FetchReference(ctx context.Context, query SpeciesReferenceQuery) (*SpeciesReference, error)
}

type iNaturalistSpeciesReferenceService struct {
	baseURL    string
	httpClient *http.Client
}

func NewSpeciesReferenceService() SpeciesReferenceService {
	return newINaturalistSpeciesReferenceService(
		defaultINaturalistAPIBaseURL,
		&http.Client{Timeout: 8 * time.Second},
	)
}

func newINaturalistSpeciesReferenceService(baseURL string, httpClient *http.Client) SpeciesReferenceService {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 8 * time.Second}
	}

	return &iNaturalistSpeciesReferenceService{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

func (s *iNaturalistSpeciesReferenceService) FetchReference(ctx context.Context, query SpeciesReferenceQuery) (*SpeciesReference, error) {
	candidates := referenceQueryCandidates(query)
	if len(candidates) == 0 {
		return nil, errSpeciesReferenceNoQuery
	}
	locale := normalizeINaturalistLocale(query.Locale)

	var lastErr error
	for _, candidate := range candidates {
		taxonID, err := s.searchTaxonID(ctx, candidate, locale)
		if err != nil {
			if errors.Is(err, ErrSpeciesReferenceNotFound) {
				lastErr = err
				continue
			}
			return nil, err
		}

		taxon, err := s.fetchTaxon(ctx, taxonID, locale)
		if err != nil {
			if errors.Is(err, ErrSpeciesReferenceNotFound) {
				lastErr = err
				continue
			}
			return nil, err
		}

		ref := taxon.toSpeciesReference(candidate)
		if ref.ScientificName == "" && strings.TrimSpace(query.ScientificName) != "" {
			ref.ScientificName = strings.TrimSpace(query.ScientificName)
		}
		if ref.CommonName == "" && strings.TrimSpace(query.CommonName) != "" {
			ref.CommonName = strings.TrimSpace(query.CommonName)
		}
		return ref, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, ErrSpeciesReferenceNotFound
}

func normalizeINaturalistLocale(raw string) string {
	locale := strings.TrimSpace(raw)
	if locale == "" || len(locale) > 16 || !iNaturalistLocalePattern.MatchString(locale) {
		return ""
	}
	return strings.ReplaceAll(locale, "_", "-")
}

func referenceQueryCandidates(query SpeciesReferenceQuery) []string {
	seen := map[string]struct{}{}
	candidates := make([]string, 0, 2)
	for _, raw := range []string{query.ScientificName, query.CommonName} {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func (s *iNaturalistSpeciesReferenceService) searchTaxonID(ctx context.Context, query string, locale string) (int, error) {
	values := url.Values{}
	values.Set("q", query)
	values.Set("rank", "species")
	values.Set("per_page", "1")
	if locale != "" {
		values.Set("locale", locale)
	}

	var response iNaturalistTaxaResponse
	if err := s.getJSON(ctx, "/taxa?"+values.Encode(), &response); err != nil {
		return 0, err
	}
	if len(response.Results) == 0 || response.Results[0].ID == 0 {
		return 0, ErrSpeciesReferenceNotFound
	}
	return response.Results[0].ID, nil
}

func (s *iNaturalistSpeciesReferenceService) fetchTaxon(ctx context.Context, taxonID int, locale string) (*iNaturalistTaxon, error) {
	values := url.Values{}
	if locale != "" {
		values.Set("locale", locale)
	}
	path := fmt.Sprintf("/taxa/%d", taxonID)
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var response iNaturalistTaxaResponse
	if err := s.getJSON(ctx, path, &response); err != nil {
		return nil, err
	}
	if len(response.Results) == 0 || response.Results[0].ID == 0 {
		return nil, ErrSpeciesReferenceNotFound
	}
	return &response.Results[0], nil
}

func (s *iNaturalistSpeciesReferenceService) getJSON(ctx context.Context, path string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Lumilio-Photos/1.0 (+https://github.com/EdwinZhanCN/Lumilio-Photos)")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ErrSpeciesReferenceNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("iNaturalist API returned status %d", resp.StatusCode)
	}

	decoder := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode iNaturalist response: %w", err)
	}
	return nil
}

type iNaturalistTaxaResponse struct {
	Results []iNaturalistTaxon `json:"results"`
}

type iNaturalistTaxon struct {
	ID                  int                     `json:"id"`
	Name                string                  `json:"name"`
	PreferredCommonName string                  `json:"preferred_common_name"`
	WikipediaSummary    string                  `json:"wikipedia_summary"`
	WikipediaURL        string                  `json:"wikipedia_url"`
	DefaultPhoto        *iNaturalistPhoto       `json:"default_photo"`
	TaxonPhotos         []iNaturalistTaxonPhoto `json:"taxon_photos"`
}

type iNaturalistTaxonPhoto struct {
	Photo *iNaturalistPhoto `json:"photo"`
}

type iNaturalistPhoto struct {
	ID              int    `json:"id"`
	URL             string `json:"url"`
	MediumURL       string `json:"medium_url"`
	LargeURL        string `json:"large_url"`
	OriginalURL     string `json:"original_url"`
	NativePageURL   string `json:"native_page_url"`
	Attribution     string `json:"attribution"`
	AttributionName string `json:"attribution_name"`
	LicenseCode     string `json:"license_code"`
}

func (t iNaturalistTaxon) toSpeciesReference(query string) *SpeciesReference {
	photo := t.bestPhoto()

	ref := &SpeciesReference{
		Provider:         speciesReferenceProviderINaturalist,
		Query:            query,
		TaxonID:          t.ID,
		ScientificName:   strings.TrimSpace(t.Name),
		CommonName:       strings.TrimSpace(t.PreferredCommonName),
		WikipediaSummary: cleanHTMLText(t.WikipediaSummary),
		WikipediaURL:     strings.TrimSpace(t.WikipediaURL),
		ReferenceURL:     fmt.Sprintf("https://www.inaturalist.org/taxa/%d", t.ID),
	}

	if photo != nil {
		ref.ImageURL = photo.bestURL()
		ref.ImageAttribution = strings.TrimSpace(photo.Attribution)
		ref.ImageLicense = strings.TrimSpace(photo.LicenseCode)
		ref.ImageSourceURL = photo.sourceURL()
	}

	return ref
}

func (t iNaturalistTaxon) bestPhoto() *iNaturalistPhoto {
	if t.DefaultPhoto != nil && t.DefaultPhoto.bestURL() != "" {
		return t.DefaultPhoto
	}
	for _, item := range t.TaxonPhotos {
		if item.Photo != nil && item.Photo.bestURL() != "" {
			return item.Photo
		}
	}
	return nil
}

func (p iNaturalistPhoto) bestURL() string {
	for _, raw := range []string{p.LargeURL, p.MediumURL, p.OriginalURL, p.URL} {
		if value := strings.TrimSpace(raw); value != "" {
			return value
		}
	}
	return ""
}

func (p iNaturalistPhoto) sourceURL() string {
	if value := strings.TrimSpace(p.NativePageURL); value != "" {
		return value
	}
	if p.ID > 0 {
		return fmt.Sprintf("https://www.inaturalist.org/photos/%d", p.ID)
	}
	return ""
}

func cleanHTMLText(raw string) string {
	cleaned := htmlTagPattern.ReplaceAllString(raw, "")
	cleaned = html.UnescapeString(cleaned)
	return strings.Join(strings.Fields(cleaned), " ")
}
