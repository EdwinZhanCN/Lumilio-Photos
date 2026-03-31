package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OllamaConfig struct {
	BaseURL    string
	Model      string
	Dimensions int
	KeepAlive  string
	Timeout    time.Duration
}

type OllamaEmbedder struct {
	config OllamaConfig
	client *http.Client
}

func NewOllamaEmbedder(config OllamaConfig) (*OllamaEmbedder, error) {
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = "http://localhost:11434"
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("ollama model is required")
	}
	if config.Dimensions <= 0 {
		return nil, fmt.Errorf("ollama dimensions must be positive")
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	return &OllamaEmbedder{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}, nil
}

func (e *OllamaEmbedder) Dimension() int {
	return e.config.Dimensions
}

func (e *OllamaEmbedder) EmbedText(ctx context.Context, text string) ([]float32, error) {
	if strings.TrimSpace(text) == "" {
		return make([]float32, e.config.Dimensions), nil
	}

	body := map[string]any{
		"model":      e.config.Model,
		"input":      text,
		"dimensions": e.config.Dimensions,
	}
	if strings.TrimSpace(e.config.KeepAlive) != "" {
		body["keep_alive"] = e.config.KeepAlive
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(e.config.BaseURL, "/")+"/api/embed",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := e.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	rawBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama embed request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(rawBody)))
	}

	var parsed struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(rawBody, &parsed); err != nil {
		return nil, err
	}
	if len(parsed.Embeddings) == 0 {
		return nil, fmt.Errorf("ollama embed response did not contain embeddings")
	}

	vector := parsed.Embeddings[0]
	if len(vector) != e.config.Dimensions {
		return nil, fmt.Errorf("ollama embed response dimension mismatch: expected=%d got=%d", e.config.Dimensions, len(vector))
	}

	return vector, nil
}
