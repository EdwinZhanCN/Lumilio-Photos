package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetricsHandlerReportsInferenceCounts(t *testing.T) {
	t.Parallel()

	metrics := &inferenceMetrics{}
	metrics.semanticImage.Add(3)
	metrics.semanticText.Add(2)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsHandler(metrics).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]uint64
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["semantic_image"] != 3 || body["semantic_text"] != 2 {
		t.Fatalf("unexpected metrics: %#v", body)
	}
}
