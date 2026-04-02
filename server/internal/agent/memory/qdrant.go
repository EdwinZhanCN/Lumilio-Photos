package memory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type QdrantConfig struct {
	BaseURL    string
	APIKey     string
	Collection string
	Distance   string
	Timeout    time.Duration
}

type QdrantStore struct {
	config QdrantConfig
	client *http.Client
}

func NewQdrantStore(config QdrantConfig) *QdrantStore {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:6333"
	}
	if config.Collection == "" {
		config.Collection = "agent_episodic_memory"
	}
	if config.Distance == "" {
		config.Distance = "Cosine"
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}

	return &QdrantStore{
		config: config,
		client: &http.Client{Timeout: config.Timeout},
	}
}

func (s *QdrantStore) EnsureCollection(ctx context.Context, dimensions int) error {
	if dimensions <= 0 {
		return errors.New("qdrant collection dimensions must be positive")
	}

	statusCode, err := s.checkCollection(ctx)
	if err == nil && statusCode == http.StatusOK {
		return nil
	}
	if err != nil && statusCode != http.StatusNotFound {
		return err
	}

	body := map[string]any{
		"vectors": map[string]any{
			"size":     dimensions,
			"distance": s.config.Distance,
		},
		"on_disk_payload": true,
		"metadata": map[string]any{
			"schema": "agent_episode_v1",
		},
	}
	return s.doJSON(ctx, http.MethodPut, "/collections/"+s.config.Collection, body, nil, http.StatusOK)
}

func (s *QdrantStore) UpsertEpisodes(ctx context.Context, episodes []Episode, vectors [][]float32) error {
	if len(episodes) == 0 {
		return nil
	}
	if len(episodes) != len(vectors) {
		return errors.New("episode and vector count mismatch")
	}

	points := make([]map[string]any, 0, len(episodes))
	for i, episode := range episodes {
		points = append(points, map[string]any{
			"id":      qdrantPointID(episode.ID),
			"vector":  vectors[i],
			"payload": payloadFromEpisode(episode),
		})
	}

	body := map[string]any{
		"points": points,
	}

	return s.doJSON(ctx, http.MethodPut, "/collections/"+s.config.Collection+"/points?wait=true", body, nil, http.StatusOK)
}

func qdrantPointID(episodeID string) string {
	if parsed, err := uuid.Parse(strings.TrimSpace(episodeID)); err == nil {
		return parsed.String()
	}
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("agent-episode:"+episodeID)).String()
}

func (s *QdrantStore) SearchEpisodes(ctx context.Context, request SearchRequest, queryVector []float32) ([]SearchHit, error) {
	limit := request.Limit
	if limit <= 0 {
		limit = 5
	}

	body := map[string]any{
		"query":        queryVector,
		"limit":        limit,
		"with_payload": true,
		"with_vector":  false,
	}
	if filter := filterFromSearchRequest(request); filter != nil {
		body["filter"] = filter
	}

	var response struct {
		Result struct {
			Points []struct {
				ID      any            `json:"id"`
				Score   float32        `json:"score"`
				Payload map[string]any `json:"payload"`
			} `json:"points"`
		} `json:"result"`
	}
	if err := s.doJSON(ctx, http.MethodPost, "/collections/"+s.config.Collection+"/points/query", body, &response, http.StatusOK); err != nil {
		return nil, err
	}

	hits := make([]SearchHit, 0, len(response.Result.Points))
	for _, point := range response.Result.Points {
		episode, err := episodeFromPayload(point.Payload)
		if err != nil {
			return nil, err
		}
		hits = append(hits, SearchHit{
			ID:      fmt.Sprint(point.ID),
			Score:   point.Score,
			Episode: episode,
			Payload: point.Payload,
		})
	}
	return hits, nil
}

func (s *QdrantStore) checkCollection(ctx context.Context) (int, error) {
	request, err := s.newRequest(ctx, http.MethodGet, "/collections/"+s.config.Collection, nil)
	if err != nil {
		return 0, err
	}

	response, err := s.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusNotFound {
		return response.StatusCode, nil
	}

	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, fmt.Errorf("qdrant get collection failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
}

func (s *QdrantStore) doJSON(ctx context.Context, method, path string, requestBody any, responseBody any, expectedStatus int) error {
	request, err := s.newRequest(ctx, method, path, requestBody)
	if err != nil {
		return err
	}

	response, err := s.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode != expectedStatus {
		return fmt.Errorf("qdrant request failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if responseBody == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, responseBody)
}

func (s *QdrantStore) newRequest(ctx context.Context, method, path string, requestBody any) (*http.Request, error) {
	var bodyReader io.Reader
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(payload)
	}

	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(s.config.BaseURL, "/")+path, bodyReader)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	if s.config.APIKey != "" {
		request.Header.Set("api-key", s.config.APIKey)
	}
	return request, nil
}

func payloadFromEpisode(episode Episode) map[string]any {
	entityNames := make([]string, 0, len(episode.Entities))
	entityTypes := make([]string, 0, len(episode.Entities))
	for _, entity := range episode.Entities {
		if entity.Name != "" {
			entityNames = append(entityNames, entity.Name)
		}
		if entity.Type != "" {
			entityTypes = append(entityTypes, entity.Type)
		}
	}

	toolNames := make([]string, 0, len(episode.ToolTrace))
	for _, step := range episode.ToolTrace {
		if step.Tool.Name != "" {
			toolNames = append(toolNames, step.Tool.Name)
		}
	}

	return map[string]any{
		"thread_id":       episode.ThreadID,
		"user_id":         episode.UserID,
		"goal":            episode.Goal,
		"intent":          episode.Intent,
		"status":          episode.Status,
		"write_trigger":   episode.WriteTrigger,
		"tags":            episode.Tags,
		"entity_names":    entityNames,
		"entity_types":    entityTypes,
		"tool_names":      toolNames,
		"workspace":       episode.Workspace,
		"route":           episode.Route,
		"started_at_unix": episode.StartedAt.Unix(),
		"ended_at_unix":   episode.EndedAt.Unix(),
		"summary":         episode.Summary,
		"retrieval_text":  episode.RetrievalText,
		"episode":         episode,
	}
}

func episodeFromPayload(payload map[string]any) (Episode, error) {
	rawEpisode, ok := payload["episode"]
	if ok {
		bytes, err := json.Marshal(rawEpisode)
		if err != nil {
			return Episode{}, err
		}
		var episode Episode
		if err := json.Unmarshal(bytes, &episode); err != nil {
			return Episode{}, err
		}
		return episode, nil
	}

	return Episode{
		ID:            stringValue(payload["id"]),
		ThreadID:      stringValue(payload["thread_id"]),
		UserID:        stringValue(payload["user_id"]),
		Goal:          stringValue(payload["goal"]),
		Intent:        stringValue(payload["intent"]),
		Summary:       stringValue(payload["summary"]),
		RetrievalText: stringValue(payload["retrieval_text"]),
		Status:        EpisodeStatus(stringValue(payload["status"])),
	}, nil
}

func filterFromSearchRequest(request SearchRequest) map[string]any {
	must := make([]map[string]any, 0, 6)

	appendMatch := func(key, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		must = append(must, map[string]any{
			"key": key,
			"match": map[string]any{
				"value": value,
			},
		})
	}

	appendMatch("user_id", request.UserID)
	appendMatch("goal", request.Goal)
	appendMatch("intent", request.Intent)
	appendMatch("entity_names", request.Entity)
	if request.Status != "" {
		appendMatch("status", string(request.Status))
	}
	if request.StartedAfter != nil || request.EndedBefore != nil {
		r := map[string]any{}
		if request.StartedAfter != nil {
			r["gte"] = request.StartedAfter.Unix()
		}
		if request.EndedBefore != nil {
			r["lte"] = request.EndedBefore.Unix()
		}
		must = append(must, map[string]any{
			"key":   "ended_at_unix",
			"range": r,
		})
	}

	if len(must) == 0 {
		return nil
	}
	return map[string]any{"must": must}
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(value)
	}
}
