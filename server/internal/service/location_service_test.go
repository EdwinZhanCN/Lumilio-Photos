package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"server/config"

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
	geocoder := newReverseGeocoder(config.GeocodingConfig{})

	require.Equal(t, geocoderProviderDisabled, geocoder.Provider())
	_, err := geocoder.Reverse(context.Background(), 0, 0)
	require.Error(t, err)
}

func TestNominatimGeocoderUsesMockEndpoint(t *testing.T) {
	var requested bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		require.Equal(t, "Lumilio-Test/1.0", r.Header.Get("User-Agent"))
		require.Equal(t, "jsonv2", r.URL.Query().Get("format"))
		require.Equal(t, "0.00000000", r.URL.Query().Get("lat"))
		require.Equal(t, "0.00000000", r.URL.Query().Get("lon"))
		fmt.Fprint(w, `{"display_name":"Null Island","address":{"country":"Ocean","state":"Equator","city":"Prime Meridian"}}`)
	}))
	defer server.Close()

	geocoder := newReverseGeocoder(config.GeocodingConfig{
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
