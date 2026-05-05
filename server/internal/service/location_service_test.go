package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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

func TestReverseGeocoderDefaultsToDisabled(t *testing.T) {
	t.Setenv("GEOCODING_PROVIDER", "")
	t.Setenv("GEOCODING_NOMINATIM_ENDPOINT", "")

	geocoder := newReverseGeocoderFromEnv(nil)

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

	t.Setenv("GEOCODING_PROVIDER", "nominatim")
	t.Setenv("GEOCODING_NOMINATIM_ENDPOINT", server.URL)
	t.Setenv("GEOCODING_LANGUAGE", "en")
	t.Setenv("GEOCODING_USER_AGENT", "Lumilio-Test/1.0")

	geocoder := newReverseGeocoderFromEnv(nil)
	result, err := geocoder.Reverse(context.Background(), 0, 0)

	require.NoError(t, err)
	require.True(t, requested)
	require.NotNil(t, result.Label)
	require.Equal(t, "Null Island", *result.Label)
	require.NotNil(t, result.Country)
	require.Equal(t, "Ocean", *result.Country)
	require.NotEmpty(t, result.RawResponse)
}

func TestNaturalEarthNameColumnsPreferLanguage(t *testing.T) {
	require.Equal(t,
		[]string{"name_zh", "name", "name_en"},
		naturalEarthNameColumns("zh-CN", "name", "name_en"),
	)
	require.Equal(t,
		[]string{"name_en", "name", "admin"},
		naturalEarthNameColumns("en", "name", "admin"),
	)
}
