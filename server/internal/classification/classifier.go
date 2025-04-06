package classification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"os"
	"sort"

	"github.com/disintegration/imaging"
	"github.com/yalue/onnxruntime_go"
)

// ImageNetClass represents a class in the ImageNet dataset
type ImageNetClass struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ClassIndex maps class indices to ImageNet classes
type ClassIndex map[string][]string

// ImageClassifier handles image classification using ONNX models
type ImageClassifier struct {
	session      *onnxruntime_go.AdvancedSession
	inputTensor  *onnxruntime_go.Tensor[float32]
	outputTensor *onnxruntime_go.Tensor[float32]
	classIndex   ClassIndex
	inputShape   []int64
}

// ClassificationResult represents the result of image classification
type ClassificationResult struct {
	ClassName  string
	Confidence float32
}

// NewImageClassifier creates a new image classifier with the given ONNX model and class index
func NewImageClassifier(modelPath, classIndexPath string) (*ImageClassifier, error) {
	//TODO: use homebrew path
	onnxruntime_go.SetSharedLibraryPath("/opt/homebrew/Cellar/onnxruntime/1.21.0/lib/libonnxruntime.dylib")

	// Initialize ONNX runtime environment if not already initialized
	err := onnxruntime_go.InitializeEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ONNX runtime environment: %w", err)
	}

	// Load the class index
	classIndex, err := loadClassIndex(classIndexPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load class index: %w", err)
	}

	// Create session options
	options, err := onnxruntime_go.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to create session options: %w", err)
	}
	defer options.Destroy()

	// Define input and output shapes
	inputShape := onnxruntime_go.NewShape(1, 3, 384, 384) // 224x224 input size for qualcomm, 300x300 for timm, 384x384 for mobilenet
	outputShape := onnxruntime_go.NewShape(1, 1000)       // Assuming 1000 classes for ImageNet

	// Create input and output tensors
	inputTensor, err := onnxruntime_go.NewEmptyTensor[float32](inputShape)
	if err != nil {
		return nil, fmt.Errorf("failed to create input tensor: %w", err)
	}

	outputTensor, err := onnxruntime_go.NewEmptyTensor[float32](outputShape)
	if err != nil {
		inputTensor.Destroy()
		return nil, fmt.Errorf("failed to create output tensor: %w", err)
	}

	// Create advanced session
	session, err := onnxruntime_go.NewAdvancedSession(
		modelPath,
		[]string{"pixel_values"}, // Input name, qualcomm: image_tensor, mobilenet: pixel_values
		[]string{"logits"},       // Output name, qualcomm: class_logits, mobilenet: logits
		[]onnxruntime_go.ArbitraryTensor{inputTensor},
		[]onnxruntime_go.ArbitraryTensor{outputTensor},
		options,
	)
	if err != nil {
		inputTensor.Destroy()
		outputTensor.Destroy()
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

	return &ImageClassifier{
		session:      session,
		inputTensor:  inputTensor,
		outputTensor: outputTensor,
		classIndex:   classIndex,
		inputShape:   inputShape,
	}, nil
}

// loadClassIndex loads the ImageNet class index from a JSON file
func loadClassIndex(path string) (ClassIndex, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open class index file: %w", err)
	}
	defer file.Close()

	var classIndex ClassIndex
	if err := json.NewDecoder(file).Decode(&classIndex); err != nil {
		return nil, fmt.Errorf("failed to decode class index: %w", err)
	}

	return classIndex, nil
}

// getInputShape returns the input shape for the model
// This is now a utility function since we define the shape explicitly when creating tensors
func getInputShape() []int64 {
	// Standard ImageNet input size: [batch_size, channels, height, width]
	return []int64{1, 3, 384, 384} //224x224 input size for qualcomm, 300x300 for timm, 384x384 for mobilenet
}

// Classify classifies an image and returns the top N results
func (c *ImageClassifier) Classify(ctx context.Context, img image.Image, topN int) ([]ClassificationResult, error) {
	// Preprocess the image and fill the input tensor
	err := c.prepareInput(img)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare input: %w", err)
	}

	// Run inference
	err = c.session.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run inference: %w", err)
	}

	// Process results
	results, err := c.processResults(topN)
	if err != nil {
		return nil, fmt.Errorf("failed to process results: %w", err)
	}

	return results, nil
}

// prepareInput preprocesses an image and fills the input tensor
func (c *ImageClassifier) prepareInput(img image.Image) error {
	width := int(c.inputShape[2])
	height := int(c.inputShape[3])
	channels := int(c.inputShape[1])
	channelSize := width * height

	// Get tensor data
	data := c.inputTensor.GetData()
	if len(data) < (channelSize * channels) {
		return fmt.Errorf("destination tensor only holds %d floats, needs %d",
			len(data), channelSize*channels)
	}

	// Get separate channels for RGB
	redChannel := data[0:channelSize]
	greenChannel := data[channelSize : channelSize*2]
	blueChannel := data[channelSize*2 : channelSize*3]
	// BGR
	//blueChannel := data[0:channelSize]
	//greenChannel := data[channelSize : channelSize*2]
	//redChannel := data[channelSize*2 : channelSize*3]

	// Resize the image to the model's input shape
	resized := imaging.Resize(img, width, height, imaging.Lanczos)

	// Fill the tensor with normalized pixel values
	i := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := resized.At(x, y).RGBA()

			// First normalize to [0, 1]
			redChannel[i] = float32(r>>8) / 255.0
			greenChannel[i] = float32(g>>8) / 255.0
			blueChannel[i] = float32(b>>8) / 255.0

			// Then apply ImageNet normalization
			redChannel[i] = (redChannel[i] - 0.485) / 0.229
			greenChannel[i] = (greenChannel[i] - 0.456) / 0.224
			blueChannel[i] = (blueChannel[i] - 0.406) / 0.225

			i++
		}
	}

	return nil
}

// processResults processes the output tensor and returns the top N results
func (c *ImageClassifier) processResults(topN int) ([]ClassificationResult, error) {
	// Get the output tensor data
	outputTensor := c.outputTensor.GetData()

	// Apply softmax to convert logits to probabilities
	softmaxScores := applySoftmax(outputTensor)

	// Create a slice of (index, score) pairs
	scores := make([]struct {
		Index int
		Score float32
	}, len(softmaxScores))

	for i, score := range softmaxScores {
		scores[i] = struct {
			Index int
			Score float32
		}{i, score}
	}

	// Sort by score in descending order
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})

	// Get the top N results
	results := make([]ClassificationResult, 0, topN)
	for i := 0; i < topN && i < len(scores); i++ {
		index := scores[i].Index
		score := scores[i].Score

		// Convert index to string
		indexStr := fmt.Sprintf("%d", index)

		// Get the class name from the class index
		classInfo, ok := c.classIndex[indexStr]
		if !ok {
			log.Printf("Warning: class index %s not found", indexStr)
			continue
		}

		results = append(results, ClassificationResult{
			ClassName:  classInfo[1],
			Confidence: score,
		})
	}

	return results, nil
}

// ClassifyFromReader classifies an image from an io.Reader
func (c *ImageClassifier) ClassifyFromReader(ctx context.Context, r io.Reader, topN int) ([]ClassificationResult, error) {
	// Decode the image
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Classify the image
	return c.Classify(ctx, img, topN)
}

// ClassifyFromFile classifies an image from a file path
func (c *ImageClassifier) ClassifyFromFile(ctx context.Context, path string, topN int) ([]ClassificationResult, error) {
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	// Classify from reader
	return c.ClassifyFromReader(ctx, file, topN)
}

// ClassifyFromBytes classifies an image from bytes
func (c *ImageClassifier) ClassifyFromBytes(ctx context.Context, data []byte, topN int) ([]ClassificationResult, error) {
	// Create a reader from bytes
	r := bytes.NewReader(data)

	// Classify from reader
	return c.ClassifyFromReader(ctx, r, topN)
}

// Close releases resources used by the classifier
func (c *ImageClassifier) Close() {
	if c.session != nil {
		c.session.Destroy()
		c.session = nil
	}
	if c.inputTensor != nil {
		c.inputTensor.Destroy()
		c.inputTensor = nil
	}
	if c.outputTensor != nil {
		c.outputTensor.Destroy()
		c.outputTensor = nil
	}
}

// applySoftmax applies the softmax function to a slice of float32 values
func applySoftmax(values []float32) []float32 {
	result := make([]float32, len(values))
	var sum float32 = 0.0

	// 找出最大值并计算exp
	var maxVal float32 = -99999
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}

	for i, v := range values {
		result[i] = float32(math.Exp(float64(v - maxVal)))
		sum += result[i]
	}

	// 归一化
	for i := range result {
		result[i] /= sum
	}

	return result
}
