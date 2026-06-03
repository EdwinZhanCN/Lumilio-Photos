package geohash

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncodeMatchesFrontendSamples(t *testing.T) {
	tests := []struct {
		name      string
		lat       float64
		lng       float64
		precision int
		want      string
	}{
		{name: "null island precision 5", lat: 0, lng: 0, precision: 5, want: "s0000"},
		{name: "null island precision 7", lat: 0, lng: 0, precision: 7, want: "s000000"},
		{name: "san francisco precision 5", lat: 37.7749, lng: -122.4194, precision: 5, want: "9q8yy"},
		{name: "san francisco precision 7", lat: 37.7749, lng: -122.4194, precision: 7, want: "9q8yyk8"},
		{name: "beijing precision 5", lat: 39.9042, lng: 116.4074, precision: 5, want: "wx4g0"},
		{name: "beijing precision 7", lat: 39.9042, lng: 116.4074, precision: 7, want: "wx4g0bm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Encode(tt.lat, tt.lng, tt.precision)

			require.True(t, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEncodeRejectsInvalidCoordinates(t *testing.T) {
	tests := []struct {
		name      string
		lat       float64
		lng       float64
		precision int
	}{
		{name: "invalid latitude", lat: 91, lng: 0, precision: 7},
		{name: "invalid longitude", lat: 0, lng: 181, precision: 7},
		{name: "nan", lat: math.NaN(), lng: 0, precision: 7},
		{name: "infinite", lat: 0, lng: math.Inf(1), precision: 7},
		{name: "invalid precision", lat: 0, lng: 0, precision: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Encode(tt.lat, tt.lng, tt.precision)

			require.False(t, ok)
			require.Empty(t, got)
		})
	}
}
