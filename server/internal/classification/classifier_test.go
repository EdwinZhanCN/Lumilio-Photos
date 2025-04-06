package classification

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestImageClassifier(t *testing.T) {
	// 设置测试模型和类索引路径
	modelPath := filepath.Join(".", "mobilenet-v4-l.onnx")
	classIndexPath := filepath.Join(".", "imagenet_class_index.json")

	// 创建分类器
	classifier, err := NewImageClassifier(modelPath, classIndexPath)
	assert.NoError(t, err)
	defer classifier.Close()

	// 测试从文件分类
	t.Run("ClassifyFromFile", func(t *testing.T) {
		// 使用data目录下的测试图片
		imgPath := filepath.Join("data", "dairy.jpg")
		_, err := os.Stat(imgPath)
		if os.IsNotExist(err) {
			t.Skip("测试图片不存在，跳过测试")
		}

		results, err := classifier.ClassifyFromFile(context.Background(), imgPath, 5)
		assert.NoError(t, err)
		assert.NotEmpty(t, results)
		t.Log("Classification Results:")
		for i, result := range results {
			t.Logf("%d. Class: %s, Confidence: %.4f", i+1, result.ClassName, result.Confidence)
		}
	})

	// 测试从字节分类
	t.Run("ClassifyFromBytes", func(t *testing.T) {
		imgPath := filepath.Join("data", "bread.jpg")
		_, err := os.Stat(imgPath)
		if os.IsNotExist(err) {
			t.Skip("测试图片不存在，跳过测试")
		}

		data, err := os.ReadFile(imgPath)
		assert.NoError(t, err)

		results, err := classifier.ClassifyFromBytes(context.Background(), data, 5)
		assert.NoError(t, err)
		assert.NotEmpty(t, results)
		assert.Greater(t, results[0].Confidence, float32(0))
	})
}
