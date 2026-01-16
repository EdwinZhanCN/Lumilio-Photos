package agent

import (
	"context"
	"fmt"
	"server/internal/db/repo"
	"sync"

	"github.com/cloudwego/eino/components/tool"
)

type ToolFactory func(ctx context.Context, deps *ToolDependencies) (tool.InvokableTool, error)

type ToolDependencies struct {
	Queries *repo.Queries // Database queries
	// LumenService *service.LumenService ont use any ai tools for now
}

type Registry struct {
	mu          sync.RWMutex
	factories   map[string]ToolFactory
	definitions map[string]string // Name -> Description (For UI)
}

func NewRegistry() *Registry {
	return &Registry{
		factories:   make(map[string]ToolFactory),
		definitions: make(map[string]string),
	}
}

func (r *Registry) BuildTools(ctx context.Context, names []string, deps *ToolDependencies) ([]tool.InvokableTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []tool.InvokableTool
	for _, name := range names {
		if factory, exists := r.factories[name]; exists {
			t, err := factory(ctx, deps)
			if err != nil {
				return nil, fmt.Errorf("failed to build tool %s: %w", name, err)
			}
			tools = append(tools, t)
		}
	}
	return tools, nil
}
