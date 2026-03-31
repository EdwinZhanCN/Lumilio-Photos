package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOllamaEmbedder(t *testing.T) {
	t.Parallel()

	type requestBody struct {
		Model      string `json:"model"`
		Input      string `json:"input"`
		Dimensions int    `json:"dimensions"`
		KeepAlive  string `json:"keep_alive"`
	}

	var received requestBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/api/embed", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&received))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"embeddings": [][]float32{{0.1, 0.2, 0.3}},
		}))
	}))
	defer server.Close()

	embedder, err := NewOllamaEmbedder(OllamaConfig{
		BaseURL:    server.URL,
		Model:      "qwen3-embedding:0.6b",
		Dimensions: 3,
		KeepAlive:  "5m",
	})
	require.NoError(t, err)

	vector, err := embedder.EmbedText(context.Background(), "find duplicate travel photos")
	require.NoError(t, err)
	require.Equal(t, []float32{0.1, 0.2, 0.3}, vector)
	require.Equal(t, 3, embedder.Dimension())
	require.Equal(t, "qwen3-embedding:0.6b", received.Model)
	require.Equal(t, "find duplicate travel photos", received.Input)
	require.Equal(t, 3, received.Dimensions)
	require.Equal(t, "5m", received.KeepAlive)
}

func TestNewOllamaEmbedderValidation(t *testing.T) {
	t.Parallel()

	_, err := NewOllamaEmbedder(OllamaConfig{BaseURL: "http://localhost:11434", Dimensions: 768})
	require.Error(t, err)

	_, err = NewOllamaEmbedder(OllamaConfig{BaseURL: "http://localhost:11434", Model: "granite-embedding:278m"})
	require.Error(t, err)
}
