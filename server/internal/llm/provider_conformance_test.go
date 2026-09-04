package llm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"server/internal/settings"

	"github.com/cloudwego/eino/schema"
)

const (
	fixtureModel      = "fixture-model"
	fixtureAPIKey     = "stored-fixture-secret"
	fixtureToolName   = "fixture_lookup"
	fixtureToolCallID = "call-fixture-1"
	fixtureText       = "fixture response"
	fixtureToolResult = "tool result accepted"
	fixtureToolArgs   = `{"query":"fixture"}`
)

func TestProviderAdaptersConformToGenerateStreamAndToolContracts(t *testing.T) {
	providers := settings.SupportedLLMProviders()
	if len(providers) != 8 {
		t.Fatalf("provider registry has %d entries, want 8", len(providers))
	}

	for _, descriptor := range providers {
		descriptor := descriptor
		t.Run(descriptor.ID, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				serveProviderFixture(t, descriptor.ID, w, r)
			}))
			defer server.Close()

			cfg := settings.LLM{
				Provider:  descriptor.ID,
				ModelName: fixtureModel,
				BaseURL:   server.URL,
			}
			if descriptor.APIKeyRequired {
				cfg.APIKey = fixtureAPIKey
			}

			chatModel, err := NewChatModel(context.Background(), cfg)
			if err != nil {
				t.Fatalf("NewChatModel() error = %v", err)
			}

			t.Run("generate", func(t *testing.T) {
				message, err := chatModel.Generate(context.Background(), []*schema.Message{
					schema.UserMessage("respond normally"),
				})
				if err != nil {
					t.Fatalf("Generate() error = %v", err)
				}
				if message.Content != fixtureText {
					t.Fatalf("Generate() content = %q, want %q", message.Content, fixtureText)
				}
				if descriptor.ID == settings.LLMProviderDeepSeek && message.ReasoningContent != "fixture reasoning" {
					t.Fatalf("DeepSeek reasoning content = %q", message.ReasoningContent)
				}
			})

			t.Run("stream", func(t *testing.T) {
				stream, err := chatModel.Stream(context.Background(), []*schema.Message{
					schema.UserMessage("respond as a stream"),
				})
				if err != nil {
					t.Fatalf("Stream() error = %v", err)
				}
				message := collectFixtureStream(t, stream)
				if message.Content != fixtureText {
					t.Fatalf("Stream() content = %q, want %q", message.Content, fixtureText)
				}
			})

			t.Run("streamed tool round trip", func(t *testing.T) {
				withTools, err := chatModel.WithTools([]*schema.ToolInfo{{
					Name: fixtureToolName,
					Desc: "Look up a deterministic fixture value.",
					ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
						"query": {Type: schema.String, Required: true},
					}),
				}})
				if err != nil {
					t.Fatalf("WithTools() error = %v", err)
				}

				userMessage := schema.UserMessage("use the fixture tool")
				stream, err := withTools.Stream(context.Background(), []*schema.Message{userMessage})
				if err != nil {
					t.Fatalf("tool Stream() error = %v", err)
				}
				toolCallMessage := collectFixtureStream(t, stream)
				if len(toolCallMessage.ToolCalls) != 1 {
					t.Fatalf("tool Stream() calls = %#v", toolCallMessage.ToolCalls)
				}
				toolCall := toolCallMessage.ToolCalls[0]
				if toolCall.Function.Name != fixtureToolName || toolCall.Function.Arguments != fixtureToolArgs {
					t.Fatalf("decoded tool call = %#v", toolCall)
				}

				toolMessage := schema.ToolMessage(`{"value":"fixture"}`, toolCall.ID)
				toolMessage.ToolName = fixtureToolName
				finalMessage, err := withTools.Generate(context.Background(), []*schema.Message{
					userMessage,
					toolCallMessage,
					toolMessage,
				})
				if err != nil {
					t.Fatalf("tool-result Generate() error = %v", err)
				}
				if finalMessage.Content != fixtureToolResult {
					t.Fatalf("tool-result content = %q, want %q", finalMessage.Content, fixtureToolResult)
				}
			})
		})
	}
}

func collectFixtureStream(t *testing.T, stream *schema.StreamReader[*schema.Message]) *schema.Message {
	t.Helper()
	defer stream.Close()

	var chunks []*schema.Message
	for {
		message, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream Recv() error = %v", err)
		}
		chunks = append(chunks, message)
	}
	if len(chunks) == 0 {
		t.Fatal("stream returned no chunks")
	}
	message, err := schema.ConcatMessages(chunks)
	if err != nil {
		t.Fatalf("ConcatMessages() error = %v", err)
	}
	return message
}

func serveProviderFixture(t *testing.T, provider string, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Errorf("read fixture request: %v", err)
		http.Error(w, "read request", http.StatusInternalServerError)
		return
	}
	if bytes.Contains(body, []byte(fixtureAPIKey)) {
		t.Errorf("provider %s placed its API key in the request body", provider)
	}
	assertFixtureAuthentication(t, provider, r)

	toolResult := fixtureRequestHasToolResult(provider, body)
	toolRequest := fixtureRequestHasTools(provider, body)
	stream := fixtureRequestIsStream(provider, r, body)

	switch provider {
	case settings.LLMProviderArk, settings.LLMProviderOpenAI, settings.LLMProviderDeepSeek,
		settings.LLMProviderQwen, settings.LLMProviderOpenRouter:
		serveOpenAICompatibleFixture(provider, w, stream, toolRequest, toolResult)
	case settings.LLMProviderOllama:
		serveOllamaFixture(w, stream, toolRequest, toolResult)
	case settings.LLMProviderClaude:
		serveClaudeFixture(w, stream, toolRequest, toolResult)
	case settings.LLMProviderGemini:
		serveGeminiFixture(w, stream, toolRequest, toolResult)
	default:
		http.Error(w, "unknown fixture provider", http.StatusBadRequest)
	}
}

func assertFixtureAuthentication(t *testing.T, provider string, r *http.Request) {
	t.Helper()
	switch provider {
	case settings.LLMProviderOllama:
		if r.Header.Get("Authorization") != "" || r.Header.Get("x-api-key") != "" {
			t.Errorf("Ollama request unexpectedly contained provider authentication")
		}
	case settings.LLMProviderClaude:
		if got := r.Header.Get("x-api-key"); got != fixtureAPIKey {
			t.Errorf("Claude x-api-key = %q", got)
		}
	case settings.LLMProviderGemini:
		if got := r.Header.Get("x-goog-api-key"); got != fixtureAPIKey {
			t.Errorf("Gemini x-goog-api-key = %q", got)
		}
	default:
		if got := r.Header.Get("Authorization"); got != "Bearer "+fixtureAPIKey {
			t.Errorf("%s Authorization header = %q", provider, got)
		}
	}
}

func fixtureRequestHasTools(provider string, body []byte) bool {
	if provider == settings.LLMProviderGemini {
		return bytes.Contains(body, []byte(`"functionDeclarations"`))
	}
	return bytes.Contains(body, []byte(`"tools"`))
}

func fixtureRequestHasToolResult(provider string, body []byte) bool {
	switch provider {
	case settings.LLMProviderClaude:
		return bytes.Contains(body, []byte(`"tool_result"`))
	case settings.LLMProviderGemini:
		return bytes.Contains(body, []byte(`"functionResponse"`))
	default:
		return bytes.Contains(body, []byte(`"role":"tool"`))
	}
}

func fixtureRequestIsStream(provider string, r *http.Request, body []byte) bool {
	if provider == settings.LLMProviderGemini {
		return strings.Contains(r.URL.Path, "streamGenerateContent")
	}
	return bytes.Contains(body, []byte(`"stream":true`))
}

func serveOpenAICompatibleFixture(provider string, w http.ResponseWriter, stream, toolRequest, toolResult bool) {
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		if toolRequest && !toolResult {
			writeSSEData(w, `{"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1,"model":"fixture-model","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call-fixture-1","type":"function","function":{"name":"fixture_lookup","arguments":"{\"query\":\"fixture\"}"}}]},"finish_reason":"tool_calls"}]}`)
			if provider == settings.LLMProviderOpenRouter {
				writeSSEData(w, `{"id":"chatcmpl-tool","object":"chat.completion.chunk","created":1,"model":"fixture-model","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			}
			writeSSEData(w, "[DONE]")
			return
		}
		writeSSEData(w, `{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1,"model":"fixture-model","choices":[{"index":0,"delta":{"role":"assistant","content":"fixture "}}]}`)
		writeSSEData(w, `{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1,"model":"fixture-model","choices":[{"index":0,"delta":{"content":"response"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
		if provider == settings.LLMProviderOpenRouter {
			writeSSEData(w, `{"id":"chatcmpl-stream","object":"chat.completion.chunk","created":1,"model":"fixture-model","choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)
		}
		writeSSEData(w, "[DONE]")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if toolResult {
		fmt.Fprint(w, openAIResponse(fixtureToolResult, ""))
		return
	}
	reasoning := ""
	if provider == settings.LLMProviderDeepSeek {
		reasoning = "fixture reasoning"
	}
	fmt.Fprint(w, openAIResponse(fixtureText, reasoning))
}

func openAIResponse(content, reasoning string) string {
	reasoningField := ""
	if reasoning != "" {
		reasoningField = `,"reasoning_content":"` + reasoning + `"`
	}
	return `{"id":"chatcmpl-generate","object":"chat.completion","created":1,"model":"fixture-model","choices":[{"index":0,"message":{"role":"assistant","content":"` + content + `"` + reasoningField + `},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`
}

func serveOllamaFixture(w http.ResponseWriter, stream, toolRequest, toolResult bool) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	if stream && toolRequest && !toolResult {
		fmt.Fprintln(w, `{"model":"fixture-model","created_at":"2026-08-18T00:00:00Z","message":{"role":"assistant","content":"","tool_calls":[{"function":{"name":"fixture_lookup","arguments":{"query":"fixture"}}}]},"done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":1}`)
		return
	}
	if stream {
		fmt.Fprintln(w, `{"model":"fixture-model","created_at":"2026-08-18T00:00:00Z","message":{"role":"assistant","content":"fixture "},"done":false}`)
		fmt.Fprintln(w, `{"model":"fixture-model","created_at":"2026-08-18T00:00:01Z","message":{"role":"assistant","content":"response"},"done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":2}`)
		return
	}
	content := fixtureText
	if toolResult {
		content = fixtureToolResult
	}
	fmt.Fprintf(w, `{"model":"fixture-model","created_at":"2026-08-18T00:00:00Z","message":{"role":"assistant","content":%q},"done":true,"done_reason":"stop","prompt_eval_count":1,"eval_count":2}`+"\n", content)
}

func serveClaudeFixture(w http.ResponseWriter, stream, toolRequest, toolResult bool) {
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		writeClaudeEvent(w, "message_start", `{"type":"message_start","message":{"id":"msg-fixture","type":"message","role":"assistant","model":"fixture-model","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":0}}}`)
		if toolRequest && !toolResult {
			writeClaudeEvent(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"call-fixture-1","name":"fixture_lookup","input":{}}}`)
			writeClaudeEvent(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"fixture\"}"}}`)
			writeClaudeEvent(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
			writeClaudeEvent(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":1}}`)
		} else {
			writeClaudeEvent(w, "content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
			writeClaudeEvent(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"fixture "}}`)
			writeClaudeEvent(w, "content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"response"}}`)
			writeClaudeEvent(w, "content_block_stop", `{"type":"content_block_stop","index":0}`)
			writeClaudeEvent(w, "message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":2}}`)
		}
		writeClaudeEvent(w, "message_stop", `{"type":"message_stop"}`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	content := fixtureText
	if toolResult {
		content = fixtureToolResult
	}
	fmt.Fprintf(w, `{"id":"msg-fixture","type":"message","role":"assistant","model":"fixture-model","content":[{"type":"text","text":%q}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":1,"output_tokens":2}}`, content)
}

func serveGeminiFixture(w http.ResponseWriter, stream, toolRequest, toolResult bool) {
	w.Header().Set("Content-Type", "application/json")
	var response string
	if toolRequest && !toolResult {
		response = `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"fixture_lookup","args":{"query":"fixture"}}}],"role":"model"},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1,"totalTokenCount":2},"modelVersion":"fixture-model","responseId":"gemini-tool"}`
	} else {
		content := fixtureText
		if toolResult {
			content = fixtureToolResult
		}
		response = fmt.Sprintf(`{"candidates":[{"content":{"parts":[{"text":%q}],"role":"model"},"finishReason":"STOP","index":0}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2,"totalTokenCount":3},"modelVersion":"fixture-model","responseId":"gemini-text"}`, content)
	}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		writeSSEData(w, response)
		return
	}
	fmt.Fprint(w, response)
}

func writeSSEData(w io.Writer, payload string) {
	fmt.Fprintf(w, "data: %s\n\n", payload)
}

func writeClaudeEvent(w io.Writer, event, payload string) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, payload)
}
