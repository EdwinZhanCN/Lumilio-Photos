package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"server/internal/db/repo"
)

const (
	geocoderProviderDisabled  = "disabled"
	geocoderProviderNominatim = "nominatim"
	defaultGeocodeLanguage    = "en"
	defaultGeocodeLimit       = 500
)

type LocationService interface {
	RebuildLocationClusters(ctx context.Context, repositoryID *string, ownerID *int32) error
	ListLocationClusters(ctx context.Context, params ListLocationClustersParams) ([]LocationCluster, int64, error)
}

type ListLocationClustersParams struct {
	RepositoryID *string
	OwnerID      *int32
	Geohash      *string
	Limit        int
	Offset       int
}

type LocationCluster struct {
	ClusterID         string
	OwnerID           int32
	RepositoryID      string
	Geohash           string
	Precision         int32
	CentroidLatitude  float64
	CentroidLongitude float64
	PhotoCount        int32
	Label             *string
	Country           *string
	Region            *string
	City              *string
	Provider          *string
	GeocodeStatus     string
	GeocodedAt        *time.Time
}

type ReverseGeocodeResult struct {
	Label       *string
	Country     *string
	Region      *string
	City        *string
	RawResponse []byte
}

type ReverseGeocoder interface {
	Provider() string
	Language() string
	Reverse(ctx context.Context, latitude, longitude float64) (ReverseGeocodeResult, error)
}

type locationService struct {
	queries  *repo.Queries
	pool     *pgxpool.Pool
	geocoder ReverseGeocoder
}

func NewLocationService(queries *repo.Queries, pool *pgxpool.Pool) LocationService {
	return &locationService{
		queries:  queries,
		pool:     pool,
		geocoder: newReverseGeocoderFromEnv(),
	}
}

func (s *locationService) RebuildLocationClusters(ctx context.Context, repositoryID *string, ownerID *int32) error {
	repositoryUUID, err := parseOptionalUUID(repositoryID)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin location cluster rebuild: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)
	scope := repo.DeleteLocationClustersForScopeParams{
		RepositoryID: repositoryUUID,
		OwnerID:      ownerID,
	}
	if err := qtx.DeleteLocationClustersForScope(ctx, scope); err != nil {
		return fmt.Errorf("delete old location clusters: %w", err)
	}
	if _, err := qtx.InsertLocationClustersForScope(ctx, repo.InsertLocationClustersForScopeParams{
		RepositoryID: repositoryUUID,
		OwnerID:      ownerID,
	}); err != nil {
		return fmt.Errorf("insert location clusters: %w", err)
	}
	if err := qtx.InsertLocationClusterAssetsForScope(ctx, repo.InsertLocationClusterAssetsForScopeParams{
		RepositoryID: repositoryUUID,
		OwnerID:      ownerID,
	}); err != nil {
		return fmt.Errorf("insert location cluster memberships: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit location cluster rebuild: %w", err)
	}

	return s.resolvePendingClusterLabels(ctx, repositoryUUID, ownerID)
}

func (s *locationService) ListLocationClusters(ctx context.Context, params ListLocationClustersParams) ([]LocationCluster, int64, error) {
	repositoryUUID, err := parseOptionalUUID(params.RepositoryID)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.queries.CountLocationClusters(ctx, repo.CountLocationClustersParams{
		RepositoryID: repositoryUUID,
		OwnerID:      params.OwnerID,
		Geohash:      normalizeOptionalText(params.Geohash),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count location clusters: %w", err)
	}

	rows, err := s.queries.ListLocationClusters(ctx, repo.ListLocationClustersParams{
		RepositoryID: repositoryUUID,
		OwnerID:      params.OwnerID,
		Geohash:      normalizeOptionalText(params.Geohash),
		Limit:        int32(params.Limit),
		Offset:       int32(params.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list location clusters: %w", err)
	}

	clusters := make([]LocationCluster, 0, len(rows))
	for _, row := range rows {
		clusters = append(clusters, toLocationCluster(row))
	}
	return clusters, total, nil
}

func (s *locationService) resolvePendingClusterLabels(ctx context.Context, repositoryID pgtype.UUID, ownerID *int32) error {
	if s.geocoder == nil || s.geocoder.Provider() == geocoderProviderDisabled {
		return s.queries.MarkLocationClustersGeocodeDisabled(ctx, repo.MarkLocationClustersGeocodeDisabledParams{
			Provider:     geocoderProviderDisabled,
			RepositoryID: repositoryID,
			OwnerID:      ownerID,
		})
	}

	clusters, err := s.queries.ListPendingLocationClusters(ctx, repo.ListPendingLocationClustersParams{
		RepositoryID: repositoryID,
		OwnerID:      ownerID,
		Limit:        defaultGeocodeLimit,
	})
	if err != nil {
		return fmt.Errorf("list pending location clusters: %w", err)
	}

	for _, cluster := range clusters {
		if err := s.resolveClusterLabel(ctx, cluster); err != nil {
			return err
		}
	}
	return nil
}

func (s *locationService) resolveClusterLabel(ctx context.Context, cluster repo.LocationCluster) error {
	provider := s.geocoder.Provider()
	language := s.geocoder.Language()
	cacheKey := fmt.Sprintf("%s:%s:%s", provider, language, cluster.Geohash)

	cached, err := s.queries.GetReverseGeocodeCache(ctx, repo.GetReverseGeocodeCacheParams{
		CacheKey: cacheKey,
		Provider: provider,
		Language: language,
	})
	if err == nil {
		return s.updateClusterGeocode(ctx, cluster.ClusterID, provider, "cached", cached.Label, cached.Country, cached.Region, cached.City)
	}
	if err != pgx.ErrNoRows {
		return fmt.Errorf("get reverse geocode cache: %w", err)
	}

	result, err := s.geocoder.Reverse(ctx, cluster.CentroidLatitude, cluster.CentroidLongitude)
	if err != nil {
		return s.updateClusterGeocode(ctx, cluster.ClusterID, provider, "failed", nil, nil, nil, nil)
	}

	cache, err := s.queries.UpsertReverseGeocodeCache(ctx, repo.UpsertReverseGeocodeCacheParams{
		CacheKey:    cacheKey,
		Provider:    provider,
		Language:    language,
		Latitude:    cluster.CentroidLatitude,
		Longitude:   cluster.CentroidLongitude,
		Label:       result.Label,
		Country:     result.Country,
		Region:      result.Region,
		City:        result.City,
		RawResponse: result.RawResponse,
	})
	if err != nil {
		return fmt.Errorf("cache reverse geocode result: %w", err)
	}

	return s.updateClusterGeocode(ctx, cluster.ClusterID, provider, "resolved", cache.Label, cache.Country, cache.Region, cache.City)
}

func (s *locationService) updateClusterGeocode(ctx context.Context, clusterID pgtype.UUID, provider, status string, label, country, region, city *string) error {
	return s.queries.UpdateLocationClusterGeocode(ctx, repo.UpdateLocationClusterGeocodeParams{
		ClusterID:     clusterID,
		Provider:      &provider,
		GeocodeStatus: status,
		Label:         label,
		Country:       country,
		Region:        region,
		City:          city,
	})
}

func parseOptionalUUID(raw *string) (pgtype.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return pgtype.UUID{}, nil
	}
	parsed, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return pgtype.UUID{Bytes: parsed, Valid: true}, nil
}

func normalizeOptionalText(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func toLocationCluster(row repo.LocationCluster) LocationCluster {
	clusterID := ""
	if row.ClusterID.Valid {
		clusterID = uuid.UUID(row.ClusterID.Bytes).String()
	}
	repositoryID := ""
	if row.RepositoryID.Valid {
		repositoryID = uuid.UUID(row.RepositoryID.Bytes).String()
	}
	var geocodedAt *time.Time
	if row.GeocodedAt.Valid {
		t := row.GeocodedAt.Time
		geocodedAt = &t
	}
	return LocationCluster{
		ClusterID:         clusterID,
		OwnerID:           row.OwnerID,
		RepositoryID:      repositoryID,
		Geohash:           row.Geohash,
		Precision:         row.Precision,
		CentroidLatitude:  row.CentroidLatitude,
		CentroidLongitude: row.CentroidLongitude,
		PhotoCount:        row.PhotoCount,
		Label:             row.Label,
		Country:           row.Country,
		Region:            row.Region,
		City:              row.City,
		Provider:          row.Provider,
		GeocodeStatus:     row.GeocodeStatus,
		GeocodedAt:        geocodedAt,
	}
}

type disabledGeocoder struct{}

func (disabledGeocoder) Provider() string { return geocoderProviderDisabled }
func (disabledGeocoder) Language() string { return "" }
func (disabledGeocoder) Reverse(context.Context, float64, float64) (ReverseGeocodeResult, error) {
	return ReverseGeocodeResult{}, fmt.Errorf("reverse geocoder disabled")
}

type nominatimGeocoder struct {
	endpoint   string
	language   string
	userAgent  string
	httpClient *http.Client
}

func newReverseGeocoderFromEnv() ReverseGeocoder {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("GEOCODING_PROVIDER")))
	endpoint := strings.TrimSpace(os.Getenv("GEOCODING_NOMINATIM_ENDPOINT"))
	if provider == "" && endpoint != "" {
		provider = geocoderProviderNominatim
	}
	if provider != geocoderProviderNominatim || endpoint == "" {
		return disabledGeocoder{}
	}
	language := strings.TrimSpace(os.Getenv("GEOCODING_LANGUAGE"))
	if language == "" {
		language = defaultGeocodeLanguage
	}
	userAgent := strings.TrimSpace(os.Getenv("GEOCODING_USER_AGENT"))
	if userAgent == "" {
		userAgent = "Lumilio-Photos/1.0"
	}
	return &nominatimGeocoder{
		endpoint:   endpoint,
		language:   language,
		userAgent:  userAgent,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (g *nominatimGeocoder) Provider() string { return geocoderProviderNominatim }
func (g *nominatimGeocoder) Language() string { return g.language }

func (g *nominatimGeocoder) Reverse(ctx context.Context, latitude, longitude float64) (ReverseGeocodeResult, error) {
	baseURL, err := url.Parse(g.endpoint)
	if err != nil {
		return ReverseGeocodeResult{}, fmt.Errorf("invalid nominatim endpoint: %w", err)
	}
	query := baseURL.Query()
	query.Set("format", "jsonv2")
	query.Set("lat", fmt.Sprintf("%.8f", latitude))
	query.Set("lon", fmt.Sprintf("%.8f", longitude))
	query.Set("zoom", "14")
	query.Set("addressdetails", "1")
	if g.language != "" {
		query.Set("accept-language", g.language)
	}
	baseURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), nil)
	if err != nil {
		return ReverseGeocodeResult{}, err
	}
	req.Header.Set("User-Agent", g.userAgent)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return ReverseGeocodeResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ReverseGeocodeResult{}, fmt.Errorf("nominatim returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ReverseGeocodeResult{}, err
	}

	var parsed struct {
		DisplayName string            `json:"display_name"`
		Name        string            `json:"name"`
		Address     map[string]string `json:"address"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ReverseGeocodeResult{}, err
	}

	label := firstNonEmpty(parsed.DisplayName, parsed.Name)
	country := firstNonEmpty(parsed.Address["country"])
	region := firstNonEmpty(parsed.Address["state"], parsed.Address["region"], parsed.Address["province"])
	city := firstNonEmpty(parsed.Address["city"], parsed.Address["town"], parsed.Address["village"], parsed.Address["municipality"], parsed.Address["county"])

	return ReverseGeocodeResult{
		Label:       emptyStringToNil(label),
		Country:     emptyStringToNil(country),
		Region:      emptyStringToNil(region),
		City:        emptyStringToNil(city),
		RawResponse: body,
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func emptyStringToNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
