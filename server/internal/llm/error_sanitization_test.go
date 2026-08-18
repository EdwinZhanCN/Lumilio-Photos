package llm

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"server/internal/settings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const (
	providerErrorSecret = "provider-error-secret"
	providerErrorPrompt = "private prompt reflected by provider"
)

func TestProviderHTTPErrorDoesNotExposeRawResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Authorization: Bearer "+providerErrorSecret+"; prompt="+providerErrorPrompt, http.StatusUnauthorized)
	}))
	defer server.Close()

	chatModel, err := NewChatModel(context.Background(), settings.LLM{
		Provider:  settings.LLMProviderOpenAI,
		APIKey:    providerErrorSecret,
		ModelName: "fixture-model",
		BaseURL:   server.URL,
	})
	if err != nil {
		t.Fatalf("NewChatModel() error = %v", err)
	}

	_, err = chatModel.Generate(context.Background(), []*schema.Message{
		schema.UserMessage(providerErrorPrompt),
	})
	assertSanitizedProviderError(t, err)
}

func TestProviderStreamReceiveErrorDoesNotExposeRawResponse(t *testing.T) {
	inner := &streamErrorChatModel{err: errors.New("raw body: " + providerErrorSecret + "; " + providerErrorPrompt)}
	chatModel := sanitizeProviderErrors(inner)

	stream, err := chatModel.Stream(context.Background(), []*schema.Message{
		schema.UserMessage(providerErrorPrompt),
	})
	if err != nil {
		t.Fatalf("Stream() setup error = %v", err)
	}
	defer stream.Close()

	_, err = stream.Recv()
	assertSanitizedProviderError(t, err)
}

func assertSanitizedProviderError(t *testing.T, err error) {
	t.Helper()
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("provider call error = %v, want sanitized failure", err)
	}
	if !errors.Is(err, ErrProviderRequest) {
		t.Fatalf("provider call error = %v, want ErrProviderRequest", err)
	}
	for _, forbidden := range []string{providerErrorSecret, providerErrorPrompt, "Authorization", "raw body"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("provider error exposed %q: %v", forbidden, err)
		}
	}
}

type streamErrorChatModel struct {
	err error
}

func (m *streamErrorChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return nil, m.err
}

func (m *streamErrorChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	stream, writer := schema.Pipe[*schema.Message](1)
	writer.Send(nil, m.err)
	writer.Close()
	return stream, nil
}

func (m *streamErrorChatModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
