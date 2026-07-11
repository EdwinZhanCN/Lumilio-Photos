package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func mapViewportContext(rawQuery string) *gin.Context {
	request := httptest.NewRequest("GET", "/api/v1/assets/map-points?"+rawQuery, nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	return context
}

func TestParseOptionalMapViewport(t *testing.T) {
	t.Run("allows an omitted viewport", func(t *testing.T) {
		south, north, west, east, err := parseOptionalMapViewport(mapViewportContext("limit=10"))
		if err != nil || south != nil || north != nil || west != nil || east != nil {
			t.Fatalf("expected empty viewport, got %v %v %v %v, err=%v", south, north, west, east, err)
		}
	})

	t.Run("parses a complete antimeridian viewport", func(t *testing.T) {
		south, north, west, east, err := parseOptionalMapViewport(
			mapViewportContext("south=-20&north=20&west=170&east=-170"),
		)
		if err != nil {
			t.Fatalf("parse viewport: %v", err)
		}
		if *south != -20 || *north != 20 || *west != 170 || *east != -170 {
			t.Fatalf("unexpected viewport: %v %v %v %v", *south, *north, *west, *east)
		}
	})

	t.Run("rejects partial or inverted latitude bounds", func(t *testing.T) {
		if _, _, _, _, err := parseOptionalMapViewport(mapViewportContext("south=-20")); err == nil {
			t.Fatal("expected partial viewport error")
		}
		if _, _, _, _, err := parseOptionalMapViewport(
			mapViewportContext("south=20&north=-20&west=-30&east=30"),
		); err == nil {
			t.Fatal("expected inverted latitude error")
		}
	})
}
