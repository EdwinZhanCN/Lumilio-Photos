package core

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
)

// ReferenceMeta 存储引用的元数据信息
type ReferenceMeta struct {
	ID          string
	Type        reflect.Type
	Description string
	CreatedAt   time.Time
}

// ReferenceManager 统一的存储容器，支持类型转换
type ReferenceManager struct {
	mu         sync.RWMutex
	vault      map[string]interface{}
	converters []TypeConverter
	deps       *ToolDependencies
	meta       map[string]*ReferenceMeta // 每个引用的元数据
}

// NewReferenceManager 创建新的 ReferenceManager 实例
func NewReferenceManager(deps *ToolDependencies) *ReferenceManager {
	rm := &ReferenceManager{
		vault: make(map[string]interface{}),
		meta:  make(map[string]*ReferenceMeta),
		deps:  deps,
	}

	// 注册默认转换器
	rm.RegisterConverter(&AssetToDTOConverter{})
	rm.RegisterConverter(&AssetToFilePathConverter{})

	return rm
}

// RegisterConverter 注册转换器
// 转换器会按照 Priority() 返回值进行排序，数字越小优先级越高
func (rm *ReferenceManager) RegisterConverter(converter TypeConverter) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 添加转换器
	rm.converters = append(rm.converters, converter)

	// 按优先级排序（插入排序：保持已排序部分的有序性）
	n := len(rm.converters)
	for i := n - 1; i > 0; i-- {
		// 如果当前转换器优先级更高（数字更小），则向前交换
		if rm.converters[i].Priority() < rm.converters[i-1].Priority() {
			rm.converters[i], rm.converters[i-1] = rm.converters[i-1], rm.converters[i]
		} else {
			// 已找到正确位置，停止排序
			break
		}
	}
}

// Store 存储数据并返回引用 ID
func (rm *ReferenceManager) Store(id string, data interface{}, description string) string {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.vault[id] = data
	rm.meta[id] = &ReferenceMeta{
		ID:          id,
		Type:        reflect.TypeOf(data),
		Description: description,
		CreatedAt:   time.Now(),
	}

	return id
}

// StoreWithID 存储数据，自动生成引用 ID
// 使用纳秒级时间戳生成 ID，如果冲突则重试（最多 10 次）
// 极端情况下回退到 UUID 以保证唯一性
func (rm *ReferenceManager) StoreWithID(data interface{}, description string) string {
	maxAttempts := 10 // 最大重试次数

	for i := 0; i < maxAttempts; i++ {
		id := fmt.Sprintf("ref_%x", time.Now().UnixNano())

		rm.mu.Lock()
		if _, exists := rm.vault[id]; !exists {
			// ID 不存在，可以使用
			rm.vault[id] = data
			rm.meta[id] = &ReferenceMeta{
				ID:          id,
				Type:        reflect.TypeOf(data),
				Description: description,
				CreatedAt:   time.Now(),
			}
			rm.mu.Unlock()
			return id
		}
		rm.mu.Unlock()

		// ID 冲突，短暂等待后重试
		time.Sleep(time.Microsecond)
	}

	// 极端情况：使用 UUID 保证唯一性
	id := fmt.Sprintf("ref_%s", uuid.New().String())
	rm.mu.Lock()
	rm.vault[id] = data
	rm.meta[id] = &ReferenceMeta{
		ID:          id,
		Type:        reflect.TypeOf(data),
		Description: description,
		CreatedAt:   time.Now(),
	}
	rm.mu.Unlock()
	return id
}

// Get 尝试获取原始数据
func (rm *ReferenceManager) Get(id string) (interface{}, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	data, ok := rm.vault[id]
	if !ok {
		return nil, fmt.Errorf("reference not found: %s", id)
	}

	return data, nil
}

// GetAs 尝试获取并转换为目标类型
func (rm *ReferenceManager) GetAs(ctx context.Context, id string, target interface{}) error {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// 1. 获取原始数据
	data, ok := rm.vault[id]
	if !ok {
		return fmt.Errorf("reference not found: %s", id)
	}

	// 2. 获取目标类型
	targetType := reflect.TypeOf(target)
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}
	sourceType := reflect.TypeOf(data)

	// 3. 检查是否可以直接赋值
	if sourceType == targetType || sourceType.AssignableTo(targetType) {
		return copier.Copy(target, data)
	}

	// 4. 尝试使用转换器进行转换
	for _, converter := range rm.converters {
		if converter.CanConvert(sourceType, targetType) {
			converted, err := converter.Convert(ctx, data, rm.deps)
			if err != nil {
				return fmt.Errorf("conversion failed: %w", err)
			}
			return copier.Copy(target, converted)
		}
	}

	// 5. 没有找到合适的转换器
	return fmt.Errorf("type mismatch: cannot convert %v to %v",
		sourceType, targetType)
}

// GetMeta 获取引用的元数据
func (rm *ReferenceManager) GetMeta(id string) (*ReferenceMeta, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	meta, ok := rm.meta[id]
	if !ok {
		return nil, fmt.Errorf("reference not found: %s", id)
	}
	return meta, nil
}

// ListRefs 列出所有引用（用于调试或 LLM 上下文）
func (rm *ReferenceManager) ListRefs() []*ReferenceMeta {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	list := make([]*ReferenceMeta, 0, len(rm.meta))
	for _, meta := range rm.meta {
		list = append(list, meta)
	}
	return list
}

// Delete 删除引用
func (rm *ReferenceManager) Delete(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.vault[id]; !ok {
		return fmt.Errorf("reference not found: %s", id)
	}

	delete(rm.vault, id)
	delete(rm.meta, id)
	return nil
}

// Clear 清空所有引用
func (rm *ReferenceManager) Clear() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.vault = make(map[string]interface{})
	rm.meta = make(map[string]*ReferenceMeta)
}

// Size 返回引用数量
func (rm *ReferenceManager) Size() int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return len(rm.vault)
}

// Reference 是一个泛型容器，解决了 JSON string -> Go Struct 的桥接问题
//
// 问题描述：
// - LLM 只能输出字符串引用 ID（如 "ref_123456"）
// - Go 结构体字段需要特定类型（如 []repo.Asset）
// - 标准 JSON 反序列化会失败：json: cannot unmarshal string into Go value of type []repo.Asset
//
// 解决方案：
// - Reference[T] 可以接收 JSON 字符串（通过 UnmarshalJSON）
// - Reference[T].Data 存储转换后的 T 类型数据
// - 中间件会在工具执行前自动填充 Data 字段
type Reference[T any] struct {
	// ID 存储 JSON 解析得到的引用 ID（如 "ref_123456"）
	ID string `json:"id"`

	// Data 存储通过 ReferenceManager 转换后的实际数据
	// 注意：此字段不由 JSON 反序列化填充，而是由中间件在运行时填充
	Data T `json:"-"`
}

// UnmarshalJSON 实现了自定义反序列化，接受两种格式的 JSON 输入：
// 1. 字符串格式："ref_123456" - LLM 直接输出的引用 ID
// 2. 对象格式：{"id": "ref_123456"} - 标准的 JSON 对象
func (r *Reference[T]) UnmarshalJSON(data []byte) error {
	var id string

	// 首先尝试解析为字符串
	if err := json.Unmarshal(data, &id); err == nil {
		r.ID = id
		return nil
	}

	// 如果不是字符串，尝试解析为对象 {"id": "..."}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	r.ID = obj.ID
	return nil
}

// Unwrap 获取数据的便捷方法，返回转换后的实际数据
// 使用示例：
//
//	assets := input.Assets.Unwrap()  // assets 的类型为 []repo.Asset
func (r *Reference[T]) Unwrap() T {
	return r.Data
}

// IsEmpty 检查引用是否为空（没有 ID 或尚未被填充数据）
func (r *Reference[T]) IsEmpty() bool {
	return r.ID == ""
}
