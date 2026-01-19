package core

import (
	"context"
	"reflect"
	"server/internal/api/dto"
	"server/internal/db/repo"
)

// TypeConverter 定义类型转换多态接口
type TypeConverter interface {
	// 检查是否支持从 fromType 转换到 toType
	CanConvert(fromType, toType reflect.Type) bool
	// 执行转换
	Convert(ctx context.Context, data interface{}, deps interface{}) (interface{}, error)
	// 获取优先级（数字越小优先级越高）
	Priority() int
}

// AssetToDTOConverter []Asset -> []AssetDTO
type AssetToDTOConverter struct{}

func (c *AssetToDTOConverter) CanConvert(fromType, toType reflect.Type) bool {
	return fromType == reflect.TypeOf([]repo.Asset{}) &&
		toType == reflect.TypeOf([]dto.AssetDTO{})
}

func (c *AssetToDTOConverter) Convert(ctx context.Context, data interface{}, deps interface{}) (interface{}, error) {
	assets := data.([]repo.Asset)
	var dtos []dto.AssetDTO
	for _, asset := range assets {
		dtos = append(dtos, dto.ToAssetDTO(asset))
	}
	return dtos, nil
}

func (c *AssetToDTOConverter) Priority() int { return 10 }

// AssetToFilePathConverter []Asset -> []string (文件路径)
type AssetToFilePathConverter struct{}

func (c *AssetToFilePathConverter) CanConvert(fromType, toType reflect.Type) bool {
	return fromType == reflect.TypeOf([]repo.Asset{}) &&
		toType == reflect.TypeOf([]string{})
}

func (c *AssetToFilePathConverter) Convert(ctx context.Context, data interface{}, deps interface{}) (interface{}, error) {
	assets := data.([]repo.Asset)
	paths := make([]string, 0, len(assets))
	for _, asset := range assets {
		if asset.StoragePath != nil {
			paths = append(paths, *asset.StoragePath)
		}
	}
	return paths, nil
}

func (c *AssetToFilePathConverter) Priority() int { return 20 }
