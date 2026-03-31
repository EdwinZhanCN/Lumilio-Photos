package memory

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

type HashEmbedder struct {
	dimensions int
}

func NewHashEmbedder(dimensions int) *HashEmbedder {
	if dimensions <= 0 {
		dimensions = 384
	}
	return &HashEmbedder{dimensions: dimensions}
}

func (e *HashEmbedder) Dimension() int {
	return e.dimensions
}

func (e *HashEmbedder) EmbedText(_ context.Context, text string) ([]float32, error) {
	vector := make([]float32, e.dimensions)
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return vector, nil
	}

	for _, token := range tokens {
		hasher := fnv.New64a()
		_, _ = hasher.Write([]byte(token))
		sum := hasher.Sum64()
		index := int(sum % uint64(e.dimensions))

		weight := float32(1)
		if sum&1 == 1 {
			weight = -1
		}
		vector[index] += weight
	}

	var norm float64
	for _, value := range vector {
		norm += float64(value * value)
	}
	if norm == 0 {
		return vector, nil
	}

	scale := float32(1 / math.Sqrt(norm))
	for i := range vector {
		vector[i] *= scale
	}

	return vector, nil
}

func tokenize(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if field != "" {
			tokens = append(tokens, field)
		}
	}
	return tokens
}
