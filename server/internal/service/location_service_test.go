package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"server/config"
	"server/internal/db"
	"server/internal/settings"

	"github.com/stretchr/testify/require"
)

func TestNormalizedGPSKeepsZeroCoordinates(t *testing.T) {
	lat := 0.0
	lng := 0.0

	gotLat, gotLng := normalizedGPS(&lat, &lng)

	require.NotNil(t, gotLat)
	require.NotNil(t, gotLng)
	require.Equal(t, 0.0, *gotLat)
	require.Equal(t, 0.0, *gotLng)
}

func TestNormalizedGPSRejectsInvalidCoordinates(t *testing.T) {
	lat := 91.0
	lng := 120.0

	gotLat, gotLng := normalizedGPS(&lat, &lng)

	require.Nil(t, gotLat)
	require.Nil(t, gotLng)
}

func TestGeohashesForGPSKeepsValidCoordinates(t *testing.T) {
	lat := 37.7749
	lng := -122.4194

	got5, got7 := geohashesForGPS(&lat, &lng)

	require.NotNil(t, got5)
	require.NotNil(t, got7)
	require.Equal(t, "9q8yy", *got5)
	require.Equal(t, "9q8yyk8", *got7)
}

func TestGeohashesForGPSRejectsInvalidCoordinates(t *testing.T) {
	lat := 91.0
	lng := 120.0
	normalizedLat, normalizedLng := normalizedGPS(&lat, &lng)

	got5, got7 := geohashesForGPS(normalizedLat, normalizedLng)

	require.Nil(t, got5)
	require.Nil(t, got7)
}

func TestReverseGeocoderDefaultsToDisabled(t *testing.T) {
	geocoder := newReverseGeocoder(settings.Geocoding{})

	require.Equal(t, geocoderProviderDisabled, geocoder.Provider())
	_, err := geocoder.Reverse(context.Background(), 0, 0)
	require.Error(t, err)
}

func TestNominatimGeocoderUsesMockEndpoint(t *testing.T) {
	var requested bool
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not permit loopback listeners: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		require.Equal(t, "Lumilio-Test/1.0", r.Header.Get("User-Agent"))
		require.Equal(t, "jsonv2", r.URL.Query().Get("format"))
		require.Equal(t, "en", r.URL.Query().Get("accept-language"))
		require.Equal(t, "0.00000000", r.URL.Query().Get("lat"))
		require.Equal(t, "0.00000000", r.URL.Query().Get("lon"))
		fmt.Fprint(w, `{"display_name":"Null Island","address":{"country":"Ocean","state":"Equator","city":"Prime Meridian"}}`)
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	geocoder := newReverseGeocoder(settings.Geocoding{
		Provider:          "nominatim",
		NominatimEndpoint: server.URL,
		Language:          "en",
		UserAgent:         "Lumilio-Test/1.0",
	})
	result, err := geocoder.Reverse(context.Background(), 0, 0)

	require.NoError(t, err)
	require.True(t, requested)
	require.NotNil(t, result.Label)
	require.Equal(t, "Null Island", *result.Label)
	require.NotNil(t, result.Country)
	require.Equal(t, "Ocean", *result.Country)
	require.NotEmpty(t, result.RawResponse)
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()

	require.Equal(t, 30*time.Second, parseRetryAfter("30"))
	require.Equal(t, time.Hour, parseRetryAfter("9223372036"))
	require.Equal(t, 0*time.Second, parseRetryAfter("not-a-date"))
}

func TestProviderRetryDelay(t *testing.T) {
	t.Parallel()

	require.Equal(t, 5*time.Second, providerRetryDelay(1, 0))
	require.Equal(t, 10*time.Second, providerRetryDelay(2, 0))
	require.Equal(t, 5*time.Minute, providerRetryDelay(10, 0))
	require.Equal(t, time.Hour, providerRetryDelay(1, 2*time.Hour))
}

func TestLocationRevisionCheckPropagatesDatabaseErrors(t *testing.T) {
	ctx := context.Background()
	catalogDir := filepath.Join(t.TempDir(), "private")
	require.NoError(t, os.Mkdir(catalogDir, 0o700))
	catalog, err := db.Open(ctx, config.DatabaseConfig{
		Path: filepath.Join(catalogDir, "catalog.sqlite3"),
	})
	require.NoError(t, err)
	require.NoError(t, catalog.Migrate(ctx))
	service := &locationService{queries: catalog.Queries}
	require.NoError(t, catalog.Close(ctx))

	_, err = service.revisionIsCurrent(ctx, 1)
	require.ErrorContains(t, err, "check geocoding revision")
}

func TestNominatimGeocoderRejectsOversizedResponse(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("sandbox does not permit loopback listeners: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, strings.Repeat("x", maxReverseGeocodeBody+1))
	}))
	server.Listener = listener
	server.Start()
	defer server.Close()

	geocoder := newReverseGeocoder(settings.Geocoding{
		Provider:          geocoderProviderNominatim,
		NominatimEndpoint: server.URL,
		Language:          "en",
		UserAgent:         settings.DefaultGeocodingUserAgent,
	})
	_, err = geocoder.Reverse(context.Background(), 1, 2)

	var providerErr *geocodeProviderError
	require.ErrorAs(t, err, &providerErr)
	require.False(t, providerErr.retryable)
}
