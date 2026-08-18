package llm

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ErrProviderRequest is deliberately free of provider response details. SDK
// errors may contain raw response bodies (and services sometimes echo request
// headers or prompts into those bodies), so only this stable classification is
// allowed to cross the LLM boundary into ordinary request logs.
var ErrProviderRequest = errors.New("llm provider request failed")

type sanitizingChatModel struct {
	inner model.ToolCallingChatModel
}

func (m *sanitizingChatModel) GetType() string {
	if typed, ok := m.inner.(interface{ GetType() string }); ok {
		return typed.GetType()
	}
	return ""
}

func sanitizeProviderErrors(inner model.ToolCallingChatModel) model.ToolCallingChatModel {
	return &sanitizingChatModel{inner: inner}
}

func sanitizeProviderError(operation string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return fmt.Errorf("%w during %s", ErrProviderRequest, operation)
}

func (m *sanitizingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	message, err := m.inner.Generate(ctx, input, opts...)
	return message, sanitizeProviderError("generate", err)
}

func (m *sanitizingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	stream, err := m.inner.Stream(ctx, input, opts...)
	if err != nil {
		return nil, sanitizeProviderError("stream", err)
	}
	if stream == nil {
		return nil, fmt.Errorf("%w during stream", ErrProviderRequest)
	}

	sanitized, writer := schema.Pipe[*schema.Message](1)
	go func() {
		defer writer.Close()
		defer stream.Close()
		for {
			message, recvErr := stream.Recv()
			if errors.Is(recvErr, io.EOF) {
				return
			}
			if recvErr != nil {
				writer.Send(nil, sanitizeProviderError("stream receive", recvErr))
				return
			}
			if writer.Send(message, nil) {
				return
			}
		}
	}()
	return sanitized, nil
}

func (m *sanitizingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	withTools, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, sanitizeProviderError("bind tools", err)
	}
	return sanitizeProviderErrors(withTools), nil
}
