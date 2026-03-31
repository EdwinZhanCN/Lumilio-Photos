package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"server/internal/agent/core"
	"server/internal/agent/memory"
	"server/internal/agent/memory/synthetic"
	mocktools "server/internal/agent/mock_tools"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "list-tools":
		err = runListTools()
	case "print-spec-example":
		err = runPrintSpecExample()
	case "seed-spec-bundle":
		err = runSeedSpecBundle(os.Args[2:])
	case "seed-episodes":
		err = runSeedEpisodes(os.Args[2:])
	case "search":
		err = runSearch(os.Args[2:])
	default:
		printUsage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "agent episodic memory harness error: %v\n", err)
		os.Exit(1)
	}
}

func runListTools() error {
	mocktools.RegisterAll()
	output := map[string]any{
		"catalog":    mocktools.Catalog(),
		"registered": core.GetRegistry().GetAllToolInfos(),
	}
	return writeJSON(output)
}

func runPrintSpecExample() error {
	return writeJSON(synthetic.ExampleSpecBundle())
}

func runSeedSpecBundle(args []string) error {
	fs := flag.NewFlagSet("seed-spec-bundle", flag.ContinueOnError)
	input := fs.String("input", "", "path to a spec bundle JSON file")
	seed := fs.Int64("seed", 42, "random seed for deterministic episode compilation")
	userID := fs.String("user", "mock-user-001", "user id used in compiled episodes")
	qdrantURL := fs.String("qdrant-url", envOrDefault("AGENT_MEMORY_QDRANT_URL", "http://localhost:6333"), "Qdrant base URL")
	qdrantAPIKey := fs.String("qdrant-api-key", os.Getenv("AGENT_MEMORY_QDRANT_API_KEY"), "Qdrant API key")
	collection := fs.String("collection", envOrDefault("AGENT_MEMORY_QDRANT_COLLECTION", "agent_episodic_memory"), "Qdrant collection")
	embedProvider := fs.String("embed-provider", envOrDefault("AGENT_MEMORY_EMBED_PROVIDER", "hash"), "embedding provider: hash or ollama")
	embedBaseURL := fs.String("embed-base-url", envOrDefault("AGENT_MEMORY_EMBED_BASE_URL", "http://localhost:11434"), "embedding service base URL for ollama")
	embedModel := fs.String("embed-model", envOrDefault("AGENT_MEMORY_EMBED_MODEL", ""), "embedding model name for ollama")
	embedDimensions := fs.Int("embed-dims", envOrDefaultInt("AGENT_MEMORY_EMBED_DIMS", 384), "embedding dimensions")
	embedKeepAlive := fs.String("embed-keep-alive", envOrDefault("AGENT_MEMORY_EMBED_KEEP_ALIVE", "5m"), "keep-alive hint for ollama embedding requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*input) == "" {
		return fmt.Errorf("input path cannot be empty")
	}

	mocktools.RegisterAll()

	bundle, err := synthetic.LoadSpecBundle(*input)
	if err != nil {
		return err
	}
	episodes, err := synthetic.CompileEpisodeSpecs(bundle.Episodes, *seed, *userID)
	if err != nil {
		return err
	}
	embedder, err := newEmbedder(*embedProvider, *embedBaseURL, *embedModel, *embedDimensions, *embedKeepAlive)
	if err != nil {
		return err
	}

	writer := memory.NewWriter(
		memory.NewQdrantStore(memory.QdrantConfig{
			BaseURL:    *qdrantURL,
			APIKey:     *qdrantAPIKey,
			Collection: *collection,
		}),
		embedder,
		memory.DefaultWritePolicy(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if err := writer.Ensure(ctx); err != nil {
		return err
	}

	written := 0
	for _, episode := range episodes {
		if err := writer.WriteEpisode(ctx, episode); err != nil {
			return err
		}
		written++
	}

	output := map[string]any{
		"input":           *input,
		"schema_version":  bundle.SchemaVersion,
		"episode_specs":   len(bundle.Episodes),
		"query_specs":     len(bundle.Queries),
		"written_count":   written,
		"embed_provider":  *embedProvider,
		"embed_model":     *embedModel,
		"collection":      *collection,
		"embed_dimension": embedder.Dimension(),
	}
	return writeJSON(output)
}

func runSeedEpisodes(args []string) error {
	fs := flag.NewFlagSet("seed-episodes", flag.ContinueOnError)
	count := fs.Int("count", 24, "number of synthetic episodes to generate")
	seed := fs.Int64("seed", 42, "random seed for repeatable episode generation")
	userID := fs.String("user", "mock-user-001", "user id used in generated episodes")
	qdrantURL := fs.String("qdrant-url", envOrDefault("AGENT_MEMORY_QDRANT_URL", "http://localhost:6333"), "Qdrant base URL")
	qdrantAPIKey := fs.String("qdrant-api-key", os.Getenv("AGENT_MEMORY_QDRANT_API_KEY"), "Qdrant API key")
	collection := fs.String("collection", envOrDefault("AGENT_MEMORY_QDRANT_COLLECTION", "agent_episodic_memory"), "Qdrant collection")
	embedProvider := fs.String("embed-provider", envOrDefault("AGENT_MEMORY_EMBED_PROVIDER", "hash"), "embedding provider: hash or ollama")
	embedBaseURL := fs.String("embed-base-url", envOrDefault("AGENT_MEMORY_EMBED_BASE_URL", "http://localhost:11434"), "embedding service base URL for ollama")
	embedModel := fs.String("embed-model", envOrDefault("AGENT_MEMORY_EMBED_MODEL", ""), "embedding model name for ollama")
	embedDimensions := fs.Int("embed-dims", envOrDefaultInt("AGENT_MEMORY_EMBED_DIMS", 384), "embedding dimensions")
	embedKeepAlive := fs.String("embed-keep-alive", envOrDefault("AGENT_MEMORY_EMBED_KEEP_ALIVE", "5m"), "keep-alive hint for ollama embedding requests")
	if err := fs.Parse(args); err != nil {
		return err
	}

	mocktools.RegisterAll()

	embedder, err := newEmbedder(*embedProvider, *embedBaseURL, *embedModel, *embedDimensions, *embedKeepAlive)
	if err != nil {
		return err
	}

	writer := memory.NewWriter(
		memory.NewQdrantStore(memory.QdrantConfig{
			BaseURL:    *qdrantURL,
			APIKey:     *qdrantAPIKey,
			Collection: *collection,
		}),
		embedder,
		memory.DefaultWritePolicy(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	if err := writer.Ensure(ctx); err != nil {
		return err
	}

	episodes := synthetic.GenerateMediaEpisodes(*count, *seed, *userID)
	written := 0
	for _, episode := range episodes {
		if err := writer.WriteEpisode(ctx, episode); err != nil {
			return err
		}
		written++
	}

	var sampleEpisode any
	if len(episodes) > 0 {
		sampleEpisode = episodes[0]
	}

	output := map[string]any{
		"embed_provider":  *embedProvider,
		"embed_model":     *embedModel,
		"collection":      *collection,
		"dimensions":      embedder.Dimension(),
		"generated_count": len(episodes),
		"written_count":   written,
		"sample_episode":  sampleEpisode,
	}
	return writeJSON(output)
}

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	query := fs.String("q", "", "search query")
	userID := fs.String("user", "", "optional user id filter")
	goal := fs.String("goal", "", "optional goal filter")
	intent := fs.String("intent", "", "optional intent filter")
	entity := fs.String("entity", "", "optional entity filter")
	status := fs.String("status", "", "optional status filter")
	tags := fs.String("tags", "", "comma-separated tags")
	limit := fs.Int("limit", 5, "number of search results to return")
	qdrantURL := fs.String("qdrant-url", envOrDefault("AGENT_MEMORY_QDRANT_URL", "http://localhost:6333"), "Qdrant base URL")
	qdrantAPIKey := fs.String("qdrant-api-key", os.Getenv("AGENT_MEMORY_QDRANT_API_KEY"), "Qdrant API key")
	collection := fs.String("collection", envOrDefault("AGENT_MEMORY_QDRANT_COLLECTION", "agent_episodic_memory"), "Qdrant collection")
	embedProvider := fs.String("embed-provider", envOrDefault("AGENT_MEMORY_EMBED_PROVIDER", "hash"), "embedding provider: hash or ollama")
	embedBaseURL := fs.String("embed-base-url", envOrDefault("AGENT_MEMORY_EMBED_BASE_URL", "http://localhost:11434"), "embedding service base URL for ollama")
	embedModel := fs.String("embed-model", envOrDefault("AGENT_MEMORY_EMBED_MODEL", ""), "embedding model name for ollama")
	embedDimensions := fs.Int("embed-dims", envOrDefaultInt("AGENT_MEMORY_EMBED_DIMS", 384), "embedding dimensions")
	embedKeepAlive := fs.String("embed-keep-alive", envOrDefault("AGENT_MEMORY_EMBED_KEEP_ALIVE", "5m"), "keep-alive hint for ollama embedding requests")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*query) == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	embedder, err := newEmbedder(*embedProvider, *embedBaseURL, *embedModel, *embedDimensions, *embedKeepAlive)
	if err != nil {
		return err
	}

	writer := memory.NewWriter(
		memory.NewQdrantStore(memory.QdrantConfig{
			BaseURL:    *qdrantURL,
			APIKey:     *qdrantAPIKey,
			Collection: *collection,
		}),
		embedder,
		memory.DefaultWritePolicy(),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hits, err := writer.Search(ctx, memory.SearchRequest{
		Query:  *query,
		UserID: *userID,
		Goal:   *goal,
		Intent: *intent,
		Entity: *entity,
		Status: memory.EpisodeStatus(*status),
		Tags:   splitCSV(*tags),
		Limit:  *limit,
	})
	if err != nil {
		return err
	}

	output := map[string]any{
		"query":          *query,
		"embed_provider": *embedProvider,
		"embed_model":    *embedModel,
		"hits":           hits,
	}
	return writeJSON(output)
}

func printUsage() {
	fmt.Print(`Usage: go run ./internal/agent <command> [flags]

Standalone mock media-assistant episodic memory harness.

Commands:
  list-tools
  print-spec-example
  seed-spec-bundle
  seed-episodes
  search

Examples:
  go run ./internal/agent list-tools
  go run ./internal/agent print-spec-example
  go run ./internal/agent seed-spec-bundle -input ./spec.bundle.json -embed-provider ollama -embed-model qwen3-embedding:0.6b -embed-dims 1024
  go run ./internal/agent seed-episodes -count 50 -embed-provider ollama -embed-model qwen3-embedding:0.6b -embed-dims 1024 -collection agent_episodic_memory
  go run ./internal/agent search -q "how did I clean up duplicate travel photos last time?" -entity Tokyo -embed-provider ollama -embed-model granite-embedding:278m -embed-dims 768
`)
}

func newEmbedder(provider, baseURL, model string, dimensions int, keepAlive string) (memory.Embedder, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "", "hash":
		return memory.NewHashEmbedder(dimensions), nil
	case "ollama":
		return memory.NewOllamaEmbedder(memory.OllamaConfig{
			BaseURL:    baseURL,
			Model:      model,
			Dimensions: dimensions,
			KeepAlive:  keepAlive,
		})
	default:
		return nil, fmt.Errorf("unsupported embed provider %q", provider)
	}
}

func writeJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envOrDefaultInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
