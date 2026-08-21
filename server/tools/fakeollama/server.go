package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

const (
	fixtureModel       = "lumilio-agent-e2e-v1"
	scenarioPrefix     = "LUMILIO_E2E_SCENARIO:"
	plainResponse      = "Deterministic Agent runtime response."
	confirmationResult = "Deterministic album update completed."
	rejectionResult    = "Deterministic album update declined."
)

type fixtureScenario struct {
	Name       string `json:"name"`
	Filename   string `json:"filename,omitempty"`
	AlbumTitle string `json:"album_title,omitempty"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   *bool           `json:"stream,omitempty"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaFunctionCall `json:"function"`
}

type ollamaFunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type ollamaTool struct {
	Type     string `json:"type"`
	Function struct {
		Name       string `json:"name"`
		Parameters struct {
			Type string `json:"type"`
		} `json:"parameters"`
	} `json:"function"`
}

type fixtureMetrics struct {
	RequestsTotal      uint64 `json:"requests_total"`
	PlainCompleted     uint64 `json:"plain_completed"`
	ConfirmationLookup uint64 `json:"confirmation_lookup"`
	ConfirmationFilter uint64 `json:"confirmation_filter"`
	ConfirmationAdd    uint64 `json:"confirmation_add"`
	ConfirmationFinal  uint64 `json:"confirmation_final"`
	ConfirmationReject uint64 `json:"confirmation_rejected"`
	SlowStarted        uint64 `json:"slow_started"`
	SlowCancelled      uint64 `json:"slow_cancelled"`
	ProviderErrors     uint64 `json:"provider_errors"`
	AuthRejections     uint64 `json:"auth_rejections"`
	ProtocolErrors     uint64 `json:"protocol_errors"`
}

type fixtureCounters struct {
	requestsTotal      atomic.Uint64
	plainCompleted     atomic.Uint64
	confirmationLookup atomic.Uint64
	confirmationFilter atomic.Uint64
	confirmationAdd    atomic.Uint64
	confirmationFinal  atomic.Uint64
	confirmationReject atomic.Uint64
	slowStarted        atomic.Uint64
	slowCancelled      atomic.Uint64
	providerErrors     atomic.Uint64
	authRejections     atomic.Uint64
	protocolErrors     atomic.Uint64
}

type fixtureServer struct {
	counters fixtureCounters
}

func newFixtureServer() *fixtureServer {
	return &fixtureServer{}
}

func (s *fixtureServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.Method == http.MethodGet && r.URL.Path == "/metrics":
		writeJSON(w, http.StatusOK, s.metrics())
	case r.Method == http.MethodPost && r.URL.Path == "/api/chat":
		s.serveChat(w, r)
	default:
		s.protocolError(w, http.StatusNotFound, "unsupported fixture endpoint")
	}
}

func (s *fixtureServer) metrics() fixtureMetrics {
	return fixtureMetrics{
		RequestsTotal:      s.counters.requestsTotal.Load(),
		PlainCompleted:     s.counters.plainCompleted.Load(),
		ConfirmationLookup: s.counters.confirmationLookup.Load(),
		ConfirmationFilter: s.counters.confirmationFilter.Load(),
		ConfirmationAdd:    s.counters.confirmationAdd.Load(),
		ConfirmationFinal:  s.counters.confirmationFinal.Load(),
		ConfirmationReject: s.counters.confirmationReject.Load(),
		SlowStarted:        s.counters.slowStarted.Load(),
		SlowCancelled:      s.counters.slowCancelled.Load(),
		ProviderErrors:     s.counters.providerErrors.Load(),
		AuthRejections:     s.counters.authRejections.Load(),
		ProtocolErrors:     s.counters.protocolErrors.Load(),
	}
}

func (s *fixtureServer) serveChat(w http.ResponseWriter, r *http.Request) {
	s.counters.requestsTotal.Add(1)
	if hasProviderAuthentication(r.Header) {
		s.counters.authRejections.Add(1)
		s.protocolError(w, http.StatusBadRequest, "provider authentication is forbidden")
		return
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	var request ollamaRequest
	if err := decoder.Decode(&request); err != nil {
		s.protocolError(w, http.StatusBadRequest, "invalid Ollama request")
		return
	}
	if request.Model != fixtureModel {
		s.protocolError(w, http.StatusBadRequest, "unknown fixture model")
		return
	}
	if request.Stream == nil || !*request.Stream {
		s.protocolError(w, http.StatusBadRequest, "fixture requires streaming")
		return
	}
	if err := validateToolShapes(request.Tools); err != nil {
		s.protocolError(w, http.StatusBadRequest, err.Error())
		return
	}

	scenario, err := scenarioFromMessages(request.Messages)
	if err != nil {
		s.protocolError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch scenario.Name {
	case "plain":
		if len(toolResults(request.Messages)) != 0 {
			s.protocolError(w, http.StatusUnprocessableEntity, "plain scenario received tool results")
			return
		}
		s.counters.plainCompleted.Add(1)
		writeTextStream(w, plainResponse)
	case "confirm-add-to-album":
		s.serveConfirmation(w, request, scenario)
	case "slow-stream":
		s.serveSlowStream(w, r.Context())
	case "provider-error":
		s.counters.providerErrors.Add(1)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "fixture-upstream-private-marker",
		})
	default:
		s.protocolError(w, http.StatusBadRequest, "unknown fixture scenario")
	}
}

func (s *fixtureServer) serveConfirmation(w http.ResponseWriter, request ollamaRequest, scenario fixtureScenario) {
	if strings.TrimSpace(scenario.Filename) == "" || strings.TrimSpace(scenario.AlbumTitle) == "" {
		s.protocolError(w, http.StatusBadRequest, "confirmation scenario requires filename and album title")
		return
	}
	if err := requireTools(request.Tools, "lookup_albums", "filter_assets", "add_to_album"); err != nil {
		s.protocolError(w, http.StatusBadRequest, err.Error())
		return
	}

	results := toolResults(request.Messages)
	switch len(results) {
	case 0:
		s.counters.confirmationLookup.Add(1)
		writeToolCall(w, "lookup_albums", map[string]any{"title_query": scenario.AlbumTitle})
	case 1:
		if results[0].name != "lookup_albums" {
			s.protocolError(w, http.StatusUnprocessableEntity, "confirmation tool order mismatch")
			return
		}
		albumID, err := albumIDFromResult(results[0].content, scenario.AlbumTitle)
		if err != nil {
			s.protocolError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		_ = albumID
		s.counters.confirmationFilter.Add(1)
		writeToolCall(w, "filter_assets", map[string]any{"filename": scenario.Filename})
	case 2:
		if results[0].name != "lookup_albums" || results[1].name != "filter_assets" {
			s.protocolError(w, http.StatusUnprocessableEntity, "confirmation tool order mismatch")
			return
		}
		albumID, err := albumIDFromResult(results[0].content, scenario.AlbumTitle)
		if err != nil {
			s.protocolError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		refID, err := refIDFromResult(results[1].content)
		if err != nil {
			s.protocolError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		s.counters.confirmationAdd.Add(1)
		writeToolCall(w, "add_to_album", map[string]any{"ref_id": refID, "album_id": albumID})
	case 3:
		if results[0].name != "lookup_albums" || results[1].name != "filter_assets" || results[2].name != "add_to_album" {
			s.protocolError(w, http.StatusUnprocessableEntity, "confirmation tool order mismatch")
			return
		}
		outcome, err := mutationOutcomeFromResult(results[2].content)
		if err != nil {
			s.protocolError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		s.counters.confirmationFinal.Add(1)
		if outcome == "rejected" {
			s.counters.confirmationReject.Add(1)
			writeTextStream(w, rejectionResult)
			return
		}
		writeTextStream(w, confirmationResult)
	default:
		s.protocolError(w, http.StatusUnprocessableEntity, "confirmation scenario has too many tool results")
	}
}

func (s *fixtureServer) serveSlowStream(w http.ResponseWriter, ctx context.Context) {
	s.counters.slowStarted.Add(1)
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"model":%q,"created_at":"2026-08-20T00:00:00Z","message":{"role":"assistant","content":"Waiting"},"done":false}`+"\n", fixtureModel)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	select {
	case <-ctx.Done():
		s.counters.slowCancelled.Add(1)
	case <-time.After(2 * time.Minute):
		writeTextChunk(w, " timed out", true)
	}
}

type namedToolResult struct {
	name    string
	content string
}

func toolResults(messages []ollamaMessage) []namedToolResult {
	results := make([]namedToolResult, 0, 3)
	lastCall := ""
	for _, message := range messages {
		if message.Role == "assistant" && len(message.ToolCalls) == 1 {
			lastCall = message.ToolCalls[0].Function.Name
			continue
		}
		if message.Role == "tool" {
			results = append(results, namedToolResult{name: lastCall, content: message.Content})
			lastCall = ""
		}
	}
	return results
}

func scenarioFromMessages(messages []ollamaMessage) (fixtureScenario, error) {
	for index := len(messages) - 1; index >= 0; index-- {
		message := messages[index]
		if message.Role != "user" {
			continue
		}
		position := strings.Index(message.Content, scenarioPrefix)
		if position < 0 {
			continue
		}
		raw := strings.TrimSpace(message.Content[position+len(scenarioPrefix):])
		var scenario fixtureScenario
		if err := json.Unmarshal([]byte(raw), &scenario); err != nil {
			return fixtureScenario{}, errors.New("invalid fixture scenario payload")
		}
		return scenario, nil
	}
	return fixtureScenario{}, errors.New("fixture scenario marker is missing")
}

func hasProviderAuthentication(headers http.Header) bool {
	for _, name := range []string{"Authorization", "X-Api-Key", "X-Goog-Api-Key"} {
		if strings.TrimSpace(headers.Get(name)) != "" {
			return true
		}
	}
	return false
}

func validateToolShapes(tools []ollamaTool) error {
	seen := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Function.Name)
		if tool.Type != "function" || name == "" || tool.Function.Parameters.Type != "object" {
			return errors.New("unknown fixture tool shape")
		}
		if _, exists := seen[name]; exists {
			return errors.New("duplicate fixture tool")
		}
		seen[name] = struct{}{}
	}
	return nil
}

func requireTools(tools []ollamaTool, names ...string) error {
	available := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		available[tool.Function.Name] = struct{}{}
	}
	for _, name := range names {
		if _, ok := available[name]; !ok {
			return fmt.Errorf("required fixture tool %s is missing", name)
		}
	}
	return nil
}

func albumIDFromResult(content, expectedTitle string) (int, error) {
	var result struct {
		Albums []struct {
			AlbumID int    `json:"album_id"`
			Title   string `json:"title"`
		} `json:"albums"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return 0, errors.New("invalid lookup_albums result")
	}
	for _, album := range result.Albums {
		// Agent lookup labels are deliberately bounded before they reach the
		// model. A long exact fixture title therefore arrives as an ellipsized
		// prefix while the stable album id remains authoritative.
		boundedTitle := strings.TrimSuffix(album.Title, "…")
		if boundedTitle != "" && strings.HasPrefix(expectedTitle, boundedTitle) && album.AlbumID > 0 {
			return album.AlbumID, nil
		}
	}
	return 0, errors.New("target album is missing from lookup result")
}

func refIDFromResult(content string) (string, error) {
	var result struct {
		Receipt *struct {
			RefID string `json:"ref_id"`
			Count int    `json:"count"`
		} `json:"receipt"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return "", errors.New("invalid filter_assets result")
	}
	if result.Receipt == nil || strings.TrimSpace(result.Receipt.RefID) == "" || result.Receipt.Count != 1 {
		return "", errors.New("filter_assets result must contain exactly one asset")
	}
	return result.Receipt.RefID, nil
}

func mutationOutcomeFromResult(content string) (string, error) {
	var result struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
		Error   any    `json:"error"`
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return "", errors.New("invalid add_to_album result")
	}
	if result.Error != nil || strings.TrimSpace(result.Message) == "" {
		return "", errors.New("add_to_album returned an error or empty message")
	}
	if result.Count == 1 {
		return "committed", nil
	}
	if result.Count == 0 && strings.Contains(strings.ToLower(result.Message), "declined") {
		return "rejected", nil
	}
	return "", errors.New("add_to_album result was neither committed nor rejected")
}

func (s *fixtureServer) protocolError(w http.ResponseWriter, status int, message string) {
	s.counters.protocolErrors.Add(1)
	log.Printf("fakeollama protocol error: status=%d message=%q", status, message)
	writeJSON(w, status, map[string]string{"error": message})
}

func writeToolCall(w http.ResponseWriter, name string, arguments map[string]any) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	writeJSONLine(w, map[string]any{
		"model":      fixtureModel,
		"created_at": "2026-08-20T00:00:00Z",
		"message": map[string]any{
			"role": "assistant", "content": "",
			"tool_calls": []any{map[string]any{"function": map[string]any{"name": name, "arguments": arguments}}},
		},
		"done": true, "done_reason": "stop", "prompt_eval_count": 1, "eval_count": 1,
	})
}

func writeTextStream(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	middle := len(content) / 2
	writeTextChunk(w, content[:middle], false)
	writeTextChunk(w, content[middle:], true)
}

func writeTextChunk(w io.Writer, content string, done bool) {
	response := map[string]any{
		"model":      fixtureModel,
		"created_at": "2026-08-20T00:00:00Z",
		"message":    map[string]any{"role": "assistant", "content": content},
		"done":       done,
	}
	if done {
		response["done_reason"] = "stop"
		response["prompt_eval_count"] = 1
		response["eval_count"] = 2
	}
	writeJSONLine(w, response)
}

func writeJSONLine(w io.Writer, value any) {
	encoded, _ := json.Marshal(value)
	_, _ = w.Write(append(encoded, '\n'))
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
