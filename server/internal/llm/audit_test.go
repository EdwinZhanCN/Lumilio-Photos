package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestAuditWrappingSurvivesToolBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "llm-audit.jsonl")
	wrapped := maybeWrapAudit(&auditFixtureModel{}, path)
	withTools, err := wrapped.WithTools([]*schema.ToolInfo{{Name: "fixture_tool"}})
	if err != nil {
		t.Fatalf("WithTools() error = %v", err)
	}

	input := []*schema.Message{schema.UserMessage("operator-audited prompt")}
	if _, err := withTools.Generate(context.Background(), input); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	stream, err := withTools.Stream(context.Background(), input)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer stream.Close()
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("stream Recv() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer file.Close()
	var entries []auditEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry auditEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("decode audit entry: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan audit log: %v", err)
	}
	if len(entries) != 2 || entries[0].Op != "generate" || entries[1].Op != "stream" {
		t.Fatalf("audit entries = %#v", entries)
	}
	for _, entry := range entries {
		if len(entry.Messages) != 1 || entry.Messages[0].Content != "operator-audited prompt" {
			t.Fatalf("audit entry lost explicit prompt: %#v", entry)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat audit log: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("audit permissions = %#o, want 0600", got)
	}
}

type auditFixtureModel struct{}

func (*auditFixtureModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage("ok", nil), nil
}

func (*auditFixtureModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage("ok", nil)}), nil
}

func (m *auditFixtureModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}
