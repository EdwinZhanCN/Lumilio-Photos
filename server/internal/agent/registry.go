package agent

import (
	"context"
	"fmt"
	"server/internal/db/repo"
	"sync"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ToolDependencies 依赖注入容器
type ToolDependencies struct {
	// 这里注入 Service 或 Repo，而不是具体的 DB 连接
	Queries *repo.Queries
}

// ToolFactory 构建函数的签名
type ToolFactory func(ctx context.Context, deps *ToolDependencies) (tool.BaseTool, error)

type Registry struct {
	mu        sync.RWMutex
	factories map[string]ToolFactory
	infos     map[string]*schema.ToolInfo // 用于给 LLM 生成描述
}

var globalRegistry *Registry
var once sync.Once

// GetRegistry 单例模式
func GetRegistry() *Registry {
	once.Do(func() {
		globalRegistry = &Registry{
			factories: make(map[string]ToolFactory),
			infos:     make(map[string]*schema.ToolInfo),
		}
	})
	return globalRegistry
}

func (r *Registry) Register(info *schema.ToolInfo, factory ToolFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[info.Name] = factory
	r.infos[info.Name] = info
}

// BuildTools 根据名称列表动态生产 Eino Tools
func (r *Registry) BuildTools(ctx context.Context, names []string, deps *ToolDependencies) ([]tool.BaseTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var tools []tool.BaseTool
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

func (r *Registry) GetAllToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

func (r *Registry) GetAllToolInfos() []*schema.ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]*schema.ToolInfo, 0, len(r.infos))
	for _, info := range r.infos {
		infos = append(infos, info)
	}
	return infos
}
