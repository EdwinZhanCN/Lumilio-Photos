package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

// ToolInputExtractor 从工具输入中提取 ref_id 并自动转换
type ToolInputExtractor struct {
	refManager *ReferenceManager
}

// NewToolInputExtractor 创建新的 ToolInputExtractor 实例
func NewToolInputExtractor(refManager *ReferenceManager) *ToolInputExtractor {
	return &ToolInputExtractor{refManager: refManager}
}

// ProcessInput 在工具执行前处理输入
// 检测输入中的 ref_id 字段，并尝试自动转换为目标类型
//
// 只支持 Reference[T] 泛型结构体标记需要解析的字段
func (e *ToolInputExtractor) ProcessInput(ctx context.Context, input interface{}) error {
	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// 只处理结构体类型
	if val.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := val.Type().Field(i)

		// 忽略不可设置的字段
		if !field.CanSet() {
			continue
		}

		// 检查是否是 Reference[T] 泛型结构体
		if field.Kind() == reflect.Struct && strings.HasSuffix(fieldType.Type.Name(), "Reference") {
			// 获取 ID 字段 (Reference[T] 的 ID)
			idField := field.FieldByName("ID")
			if !idField.IsValid() || idField.Kind() != reflect.String {
				continue
			}

			refID := idField.String()
			if refID == "" {
				continue
			}

			// 填充 Data 字段
			dataField := field.FieldByName("Data")
			if dataField.IsValid() && dataField.CanSet() {
				// 创建 Data 字段目标类型的实例
				targetData := reflect.New(dataField.Type()).Interface()

				// 调用 ReferenceManager 进行转换
				err := e.refManager.GetAs(ctx, refID, targetData)
				if err != nil {
					return fmt.Errorf("reference resolution failed for field %s: %w", fieldType.Name, err)
				}

				// 将转换后的数据塞回 Reference[T].Data
				dataField.Set(reflect.ValueOf(targetData).Elem())
			}
		}
	}
	return nil
}

// isReferenceID 检查字符串是否是引用 ID
func (e *ToolInputExtractor) isReferenceID(s string) bool {
	// 空字符串不是引用
	if s == "" {
		return false
	}

	// ref_ 前缀格式（系统生成的引用 ID）
	if strings.HasPrefix(s, "ref_") && len(s) > 4 {
		return true
	}

	// UUID 格式 (标准 UUID: 8-4-4-4-12)
	if len(s) == 36 && s[8] == '-' && s[13] == '-' && s[18] == '-' && s[23] == '-' {
		// 简单格式检查，不使用正则表达式提高性能
		return true
	}

	return false
}

// ExtractReferences 从输入中提取所有引用 ID
//
// 使用显式标记：只提取 Reference[T] 类型字段的 ID 字段
func (e *ToolInputExtractor) ExtractReferences(input interface{}) []string {
	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	var refs []string

	if val.Kind() != reflect.Struct {
		return refs
	}

	// 遍历所有字段
	for i := 0; i < val.NumField(); i++ {
		fieldVal := val.Field(i)
		fieldType := val.Type().Field(i)

		if !fieldVal.CanInterface() {
			continue
		}

		// 检查是否是 Reference[T] 类型
		if fieldVal.Kind() == reflect.Struct && strings.HasSuffix(fieldType.Type.Name(), "Reference") {
			idField := fieldVal.FieldByName("ID")
			if idField.IsValid() && idField.Kind() == reflect.String {
				refID := idField.String()
				if refID != "" && e.isReferenceID(refID) {
					refs = append(refs, refID)
				}
			}
		}
	}

	return refs
}

// GetFieldType 获取字段的反射类型（用于调试）
func (e *ToolInputExtractor) GetFieldType(input interface{}, fieldName string) (reflect.Type, error) {
	val := reflect.ValueOf(input)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input is not a struct")
	}

	field, found := val.Type().FieldByName(fieldName)
	if !found {
		return nil, fmt.Errorf("field %s not found", fieldName)
	}

	return field.Type, nil
}
