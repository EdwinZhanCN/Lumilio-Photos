package llm

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

var auditMu sync.Mutex

// maybeWrapAudit returns the model unchanged unless the host supplied an audit
// path as an explicit single-run operator control.
func maybeWrapAudit(inner model.ToolCallingChatModel, path string) model.ToolCallingChatModel {
	if path == "" {
		return inner
	}
	return &auditingChatModel{inner: inner, path: path}
}

type auditingChatModel struct {
	inner model.ToolCallingChatModel
	path  string
}

type auditEntry struct {
	Timestamp time.Time         `json:"ts"`
	Op        string            `json:"op"`
	Messages  []*schema.Message `json:"messages"`
}

func (m *auditingChatModel) log(op string, input []*schema.Message) {
	line, err := json.Marshal(auditEntry{Timestamp: time.Now(), Op: op, Messages: input})
	if err != nil {
		return
	}
	auditMu.Lock()
	defer auditMu.Unlock()
	f, err := os.OpenFile(m.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(line, '\n'))
}

func (m *auditingChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.log("generate", input)
	return m.inner.Generate(ctx, input, opts...)
}

func (m *auditingChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	m.log("stream", input)
	return m.inner.Stream(ctx, input, opts...)
}

func (m *auditingChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	withTools, err := m.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	return &auditingChatModel{inner: withTools, path: m.path}, nil
}
