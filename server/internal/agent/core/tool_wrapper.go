package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ToolWrapper 包装工具并自动应用中间件
//
// 用途：在工具执行前后插入自定义逻辑，例如：
// - 输入预处理：解析 Reference[T] 泛型的 ref_id
// - 输出后处理：转换返回值格式
// - 日志记录：记录工具调用参数和结果
// - 性能监控：统计工具执行时间
type ToolWrapper struct {
	originalTool   tool.BaseTool
	inputExtractor *ToolInputExtractor
	info           *schema.ToolInfo
}

// NewToolWrapper 创建工具包装器
func NewToolWrapper(originalTool tool.BaseTool, extractor *ToolInputExtractor) tool.BaseTool {
	// 提前获取 ToolInfo（避免每次调用时都获取）
	info, _ := originalTool.Info(context.Background())

	return &ToolWrapper{
		originalTool:   originalTool,
		inputExtractor: extractor,
		info:           info,
	}
}

// Info 实现 tool.BaseTool 接口
func (w *ToolWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	if w.info != nil {
		return w.info, nil
	}
	return w.originalTool.Info(ctx)
}

// InvokableRun 实现 tool.InvokableTool 接口
//
// 执行流程：
// 1. 将 JSON 参数反序列化为 map
// 2. 前置处理：调用 InputExtractor.ProcessInput() 解析 Reference[T]
// 3. 工具执行：调用原始工具的 InvokableRun 方法
// 4. 返回结果：将工具执行结果返回给调用方
func (w *ToolWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 阶段 1: 解析 JSON 参数为 map
	var argsMap map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &argsMap); err != nil {
		return "", fmt.Errorf("failed to parse arguments JSON: %w", err)
	}

	// 阶段 2: 在工具执行前处理输入（解析 Reference[T]）
	if w.inputExtractor != nil {
		if err := w.inputExtractor.ProcessInput(ctx, argsMap); err != nil {
			return "", fmt.Errorf("tool input preprocessing failed: %w", err)
		}
	}

	// 阶段 3: 将处理后的参数重新序列化为 JSON
	processedJSON, err := json.Marshal(argsMap)
	if err != nil {
		return "", fmt.Errorf("failed to serialize processed arguments: %w", err)
	}

	// 阶段 4: 执行原始工具
	if invokable, ok := w.originalTool.(tool.InvokableTool); ok {
		return invokable.InvokableRun(ctx, string(processedJSON), opts...)
	}

	return "", fmt.Errorf("wrapped tool is not invokable")
}
