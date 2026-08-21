package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"server/internal/llm"
	"server/internal/settings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestFixturePlainStreamUsesPinnedEinoOllamaDialect(t *testing.T) {
	model := fixtureChatModel(t)
	message := collectStream(t, streamModel(t, model, []*schema.Message{
		schema.UserMessage(scenarioPrompt(fixtureScenario{Name: "plain"})),
	}))
	if message.Content != plainResponse {
		t.Fatalf("plain response = %q, want %q", message.Content, plainResponse)
	}
}

func TestFixtureConfirmationToolRoundTripUsesPinnedEinoOllamaDialect(t *testing.T) {
	chatModel := fixtureChatModel(t)
	withTools, err := chatModel.WithTools([]*schema.ToolInfo{
		fixtureTool("lookup_albums", "title_query"),
		fixtureTool("filter_assets", "filename"),
		fixtureTool("add_to_album", "ref_id", "album_id"),
	})
	if err != nil {
		t.Fatalf("WithTools() error = %v", err)
	}

	user := schema.UserMessage(scenarioPrompt(fixtureScenario{
		Name: "confirm-add-to-album", Filename: "fixture.jpg", AlbumTitle: "Fixture Album",
	}))
	messages := []*schema.Message{user}

	lookup := collectStream(t, streamModel(t, withTools, messages))
	assertToolCall(t, lookup, "lookup_albums", `{"title_query":"Fixture Album"}`)
	messages = append(messages, lookup, schema.ToolMessage(`{"albums":[{"album_id":42,"title":"Fixture Album","asset_count":0}]}`, ""))

	filter := collectStream(t, streamModel(t, withTools, messages))
	assertToolCall(t, filter, "filter_assets", `{"filename":"fixture.jpg"}`)
	messages = append(messages, filter, schema.ToolMessage(`{"receipt":{"ref_id":"fixture-ref","count":1,"summary":"one asset"}}`, ""))

	add := collectStream(t, streamModel(t, withTools, messages))
	assertToolCall(t, add, "add_to_album", `{"album_id":42,"ref_id":"fixture-ref"}`)
	messages = append(messages, add, schema.ToolMessage(`{"message":"Added 1 photos to album","album_id":42,"count":1}`, ""))

	final := collectStream(t, streamModel(t, withTools, messages))
	if final.Content != confirmationResult {
		t.Fatalf("confirmation response = %q, want %q", final.Content, confirmationResult)
	}
}

func TestFixtureConfirmationRejectionUsesPinnedEinoOllamaDialect(t *testing.T) {
	chatModel := fixtureChatModel(t)
	withTools, err := chatModel.WithTools([]*schema.ToolInfo{
		fixtureTool("lookup_albums", "title_query"),
		fixtureTool("filter_assets", "filename"),
		fixtureTool("add_to_album", "ref_id", "album_id"),
	})
	if err != nil {
		t.Fatalf("WithTools() error = %v", err)
	}

	messages := []*schema.Message{schema.UserMessage(scenarioPrompt(fixtureScenario{
		Name: "confirm-add-to-album", Filename: "fixture.jpg", AlbumTitle: "Fixture Album",
	}))}
	lookup := collectStream(t, streamModel(t, withTools, messages))
	messages = append(messages, lookup, schema.ToolMessage(`{"albums":[{"album_id":42,"title":"Fixture Album","asset_count":0}]}`, ""))
	filter := collectStream(t, streamModel(t, withTools, messages))
	messages = append(messages, filter, schema.ToolMessage(`{"receipt":{"ref_id":"fixture-ref","count":1,"summary":"one asset"}}`, ""))
	add := collectStream(t, streamModel(t, withTools, messages))
	messages = append(messages, add, schema.ToolMessage(`{"message":"Album update was not applied: the user declined."}`, ""))

	final := collectStream(t, streamModel(t, withTools, messages))
	if final.Content != rejectionResult {
		t.Fatalf("rejection response = %q, want %q", final.Content, rejectionResult)
	}
}

func TestFixtureRejectsProviderAuthenticationWithoutEchoingIt(t *testing.T) {
	for _, header := range []string{"Authorization", "X-Api-Key", "X-Goog-Api-Key"} {
		t.Run(header, func(t *testing.T) {
			server := httptest.NewServer(newFixtureServer())
			defer server.Close()
			body := scenarioPrompt(fixtureScenario{Name: "plain"})
			request, err := http.NewRequest(http.MethodPost, server.URL+"/api/chat", strings.NewReader(`{"model":"`+fixtureModel+`","messages":[{"role":"user","content":`+mustJSON(body)+`}],"stream":true}`))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set(header, "must-not-leak")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			responseBody, _ := io.ReadAll(response.Body)
			if response.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusBadRequest)
			}
			if strings.Contains(string(responseBody), "must-not-leak") {
				t.Fatalf("response echoed provider secret: %s", responseBody)
			}
		})
	}
}

func TestFixtureFailsClosedForUnknownProtocol(t *testing.T) {
	plainPrompt := mustJSON(scenarioPrompt(fixtureScenario{Name: "plain"}))
	unknownPrompt := mustJSON(scenarioPrompt(fixtureScenario{Name: "unknown"}))
	confirmationPrompt := mustJSON(scenarioPrompt(fixtureScenario{
		Name: "confirm-add-to-album", Filename: "fixture.jpg", AlbumTitle: "Fixture Album",
	}))
	tools := `[
		{"type":"function","function":{"name":"lookup_albums","parameters":{"type":"object"}}},
		{"type":"function","function":{"name":"filter_assets","parameters":{"type":"object"}}},
		{"type":"function","function":{"name":"add_to_album","parameters":{"type":"object"}}}
	]`
	testCases := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{
			name: "unknown endpoint", method: http.MethodGet, path: "/unknown",
			status: http.StatusNotFound,
		},
		{
			name: "unknown model", method: http.MethodPost, path: "/api/chat",
			body:   `{"model":"other","messages":[{"role":"user","content":` + plainPrompt + `}],"stream":true}`,
			status: http.StatusBadRequest,
		},
		{
			name: "non-streaming request", method: http.MethodPost, path: "/api/chat",
			body:   `{"model":"` + fixtureModel + `","messages":[{"role":"user","content":` + plainPrompt + `}],"stream":false}`,
			status: http.StatusBadRequest,
		},
		{
			name: "unknown scenario", method: http.MethodPost, path: "/api/chat",
			body:   `{"model":"` + fixtureModel + `","messages":[{"role":"user","content":` + unknownPrompt + `}],"stream":true}`,
			status: http.StatusBadRequest,
		},
		{
			name: "unknown tool shape", method: http.MethodPost, path: "/api/chat",
			body: `{"model":"` + fixtureModel + `","messages":[{"role":"user","content":` + plainPrompt + `}],"stream":true,` +
				`"tools":[{"type":"not-a-function","function":{"name":"lookup_albums","parameters":{"type":"object"}}}]}`,
			status: http.StatusBadRequest,
		},
		{
			name: "out-of-order tool result", method: http.MethodPost, path: "/api/chat",
			body: `{"model":"` + fixtureModel + `","stream":true,"tools":` + tools + `,"messages":[` +
				`{"role":"user","content":` + confirmationPrompt + `},` +
				`{"role":"assistant","content":"","tool_calls":[{"function":{"name":"filter_assets","arguments":{"filename":"fixture.jpg"}}}]},` +
				`{"role":"tool","content":"{\"receipt\":{\"ref_id\":\"fixture-ref\",\"count\":1}}"}]}`,
			status: http.StatusUnprocessableEntity,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newFixtureServer()
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			server.ServeHTTP(recorder, request)
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d, want %d; body=%s", recorder.Code, testCase.status, recorder.Body.String())
			}
			if server.metrics().ProtocolErrors != 1 {
				t.Fatalf("protocol errors = %d, want 1", server.metrics().ProtocolErrors)
			}
			if strings.Contains(recorder.Body.String(), plainResponse) {
				t.Fatalf("invalid protocol fell back to a normal response: %s", recorder.Body.String())
			}
		})
	}
}

func TestFixtureProtocolErrorsNeverLogRequestBodies(t *testing.T) {
	var logs bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	log.SetOutput(&logs)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	})

	const privateBodyMarker = "body-private-marker-must-not-be-logged"
	server := newFixtureServer()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/chat",
		strings.NewReader(`{"model":"`+privateBodyMarker),
	)
	server.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if strings.Contains(logs.String(), privateBodyMarker) {
		t.Fatalf("protocol log exposed request body: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "invalid Ollama request") {
		t.Fatalf("protocol log omitted bounded error identity: %s", logs.String())
	}
}

func TestAlbumIDFromResultAcceptsBoundedLookupLabel(t *testing.T) {
	title := "Agent Runtime e2e-w0 reject-1787281306541-0-0"
	albumID, err := albumIDFromResult(
		`{"albums":[{"album_id":42,"title":"Agent Runtime e2e-w0 reject-1787281306…","asset_count":0}]}`,
		title,
	)
	if err != nil {
		t.Fatalf("albumIDFromResult() error = %v", err)
	}
	if albumID != 42 {
		t.Fatalf("album id = %d, want 42", albumID)
	}
}

func fixtureChatModel(t *testing.T) model.ToolCallingChatModel {
	t.Helper()
	server := httptest.NewServer(newFixtureServer())
	t.Cleanup(server.Close)
	chatModel, err := llm.NewChatModel(context.Background(), settings.LLM{
		Provider: settings.LLMProviderOllama, ModelName: fixtureModel, BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewChatModel() error = %v", err)
	}
	return chatModel
}

func fixtureTool(name string, params ...string) *schema.ToolInfo {
	properties := make(map[string]*schema.ParameterInfo, len(params))
	for _, param := range params {
		properties[param] = &schema.ParameterInfo{Type: schema.String, Required: true}
	}
	return &schema.ToolInfo{Name: name, Desc: name, ParamsOneOf: schema.NewParamsOneOfByParams(properties)}
}

func scenarioPrompt(scenario fixtureScenario) string {
	encoded, _ := json.Marshal(scenario)
	return scenarioPrefix + string(encoded)
}

type streamingModel interface {
	Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error)
}

func streamModel(t *testing.T, chatModel streamingModel, messages []*schema.Message) *schema.StreamReader[*schema.Message] {
	t.Helper()
	stream, err := chatModel.Stream(context.Background(), messages)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	return stream
}

func collectStream(t *testing.T, stream *schema.StreamReader[*schema.Message]) *schema.Message {
	t.Helper()
	defer stream.Close()
	var chunks []*schema.Message
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Recv() error = %v", err)
		}
		chunks = append(chunks, message)
	}
	message, err := schema.ConcatMessages(chunks)
	if err != nil {
		t.Fatalf("ConcatMessages() error = %v", err)
	}
	return message
}

func assertToolCall(t *testing.T, message *schema.Message, name, arguments string) {
	t.Helper()
	if len(message.ToolCalls) != 1 || message.ToolCalls[0].Function.Name != name {
		t.Fatalf("tool calls = %#v, want %s", message.ToolCalls, name)
	}
	var got, want any
	if err := json.Unmarshal([]byte(message.ToolCalls[0].Function.Arguments), &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(arguments), &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("tool arguments = %s, want %s", gotJSON, wantJSON)
	}
}

func mustJSON(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
