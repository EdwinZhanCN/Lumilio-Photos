# Agent Memory Research

This directory contains the reproducible research harness for the media-agent episodic memory experiment.

The boundary is deliberate:

- Go under `server/internal/agent` runs the runtime-facing harness, writes episodes, and seeds Qdrant.
- Python under `server/internal/agent/research` generates datasets, validates them, makes splits, and benchmarks retrieval.
- JSON Schema under `server/internal/agent/schemas` is the shared contract between both sides.

## Goal

The experiment is not a general chatbot benchmark.
It measures whether a media-management agent can retrieve prior task episodes in a RAG-like way.

Current scope:

- synthetic media-management episodes
- Qdrant for vector retrieval
- Ollama-hosted embedding models
- DeepSeek for dataset generation
- baseline comparison between `qwen3-embedding:0.6b` and `granite-embedding:278m`

## Layout

- `src/agent_memory_research/cli.py`: Python CLI entrypoint
- `src/agent_memory_research/deepseek_client.py`: DeepSeek dataset generation
- `src/agent_memory_research/qdrant_client.py`: Qdrant query client for benchmarking
- `src/agent_memory_research/ollama_embedder.py`: Ollama embedding client
- `src/agent_memory_research/bundle_lint.py`: research-specific lint rules
- `data/raw/`: generated spec bundles
- `data/splits/`: train/val/test bundles and manifests
- `data/reports/`: retrieval benchmark reports
- `../schemas/episode_spec_bundle.schema.json`: episode/query bundle schema
- `../schemas/dataset_manifest.schema.json`: split manifest schema
- `../schemas/retrieval_benchmark_report.schema.json`: benchmark report schema

## Reproducibility Contract

Research data flows through one shared contract:

- `episode_spec_bundle.schema.json`
- `dataset_manifest.schema.json`
- `retrieval_benchmark_report.schema.json`

Important normalization and evaluation rules in the current harness:

- Every episode is expected to have a stable `episode_id`.
- Every query is expected to carry `target_episode_ids` for instance-level evaluation.
- Each generation batch is now driven by a programmatic generation matrix before calling DeepSeek.
- DeepSeek output is normalized before schema validation. Numeric metadata values are coerced to strings.
- Missing `episode_id` or `target_episode_ids` can be backfilled locally for compatibility, but new datasets should emit them explicitly.
- The default protocol is now `episodes first, queries second`: generate episodes, split by cluster, then generate split-specific query sets.
- Split bundles may validly contain zero queries before split-specific query generation.
- Benchmark filtering uses exact constraints for `entity` and `status`, but does not hard-filter on `tags`.
- Different embedding models or dimensions should use different Qdrant collections.

## Prerequisites

### 1. Python research environment

```bash
cd /Users/zhanzihao/Lumilio-Photos/server/internal/agent/research
uv sync
```

### 2. DeepSeek API key

```bash
export DEEPSEEK_API_KEY=...
```

### 3. Ollama

```bash
ollama pull qwen3-embedding:0.6b
ollama pull granite-embedding:278m
ollama serve
```

Optional sanity checks:

```bash
curl http://localhost:11434/api/tags
curl http://localhost:11434/api/embed -d '{
  "model": "qwen3-embedding:0.6b",
  "input": "show me previous duplicate cleanup episodes"
}'
```

### 4. Qdrant

Start a local or remote Qdrant instance and set:

```bash
export AGENT_MEMORY_QDRANT_URL=http://localhost:6333
export AGENT_MEMORY_QDRANT_COLLECTION=agent_episodic_memory
```

If needed:

```bash
export AGENT_MEMORY_QDRANT_API_KEY=...
```

Sanity check:

```bash
curl http://localhost:6333/collections
```

## CLI Overview

```bash
cd /Users/zhanzihao/Lumilio-Photos/server/internal/agent/research
uv run agent-memory-research print-schema-path
uv run agent-memory-research print-generation-plan --episode-count 12 --query-count 4 --seed 42
uv run agent-memory-research validate-bundle data/raw/example.bundle.json
uv run agent-memory-research generate-spec-bundle data/raw/spec.bundle.json
uv run agent-memory-research make-splits data/raw/spec.bundle.json data/splits/spec
uv run agent-memory-research generate-split-queries data/splits/spec/train.bundle.json
uv run agent-memory-research generate-split-queries data/splits/spec/val.bundle.json
uv run agent-memory-research generate-split-queries data/splits/spec/test.bundle.json
uv run agent-memory-research benchmark-retrieval \
  data/splits/spec/test.bundle.json \
  --collection agent_episodic_memory_qwen3_1024 \
  --embed-model qwen3-embedding:0.6b \
  --embed-dims 1024 \
  --output-path data/reports/qwen3-spec.json
uv run agent-memory-research analyze-benchmark-report \
  data/reports/qwen3-spec.json
uv run agent-memory-research analyze-benchmark-report \
  data/reports/qwen3-spec.json \
  --output-path data/reports/qwen3-spec-analysis.md
```

## Current Protocol

The benchmark construction protocol is now:

1. generate the full episode corpus
2. split episodes into `train`, `val`, and `test` by `cluster`
3. generate queries independently for each split
4. freeze the test split before model development

This means `make-splits` now produces episode-only split bundles with empty `queries` arrays. Split-specific queries are added in a later step with `generate-split-queries`.

## Reproducing the Current Workflow

### Step 1. Generate the raw bundle

```bash
cd /Users/zhanzihao/Lumilio-Photos/server/internal/agent/research
uv run agent-memory-research generate-spec-bundle data/raw/baseline-v1.bundle.json \
  --episode-count 35 \
  --batch-episode-count 7 \
  --max-tokens 5500 \
  --timeout-seconds 180 \
  --model deepseek-chat
```

What this does:

- builds a deterministic batch-level generation matrix first
- generates media-management episodes in batches
- validates every batch against schema
- writes a final validated bundle to `data/raw/baseline-v1.bundle.json`

### Step 2. Validate the raw bundle

```bash
cd /Users/zhanzihao/Lumilio-Photos/server/internal/agent/research
uv run agent-memory-research validate-bundle data/raw/baseline-v1.bundle.json
```

### Step 3. Create train/val/test splits

```bash
cd /Users/zhanzihao/Lumilio-Photos/server/internal/agent/research
uv run agent-memory-research make-splits \
  data/raw/baseline-v1.bundle.json \
  data/splits/baseline-v1
```

Outputs:

- `data/splits/baseline-v1/train.bundle.json`
- `data/splits/baseline-v1/val.bundle.json`
- `data/splits/baseline-v1/test.bundle.json`
- `data/splits/baseline-v1/manifest.json`

At this point the split bundles contain episodes only. Queries are generated independently per split in the next step.

### Step 4. Generate split-specific query sets

```bash
cd /Users/zhanzihao/Lumilio-Photos/server/internal/agent/research
uv run agent-memory-research generate-split-queries data/splits/baseline-v1/train.bundle.json
uv run agent-memory-research generate-split-queries data/splits/baseline-v1/val.bundle.json
uv run agent-memory-research generate-split-queries data/splits/baseline-v1/test.bundle.json
```

### Step 5. Seed Qwen3 embeddings into Qdrant

Use a dedicated collection for this model/dimension pair.

```bash
cd /Users/zhanzihao/Lumilio-Photos/server
go run ./internal/agent seed-spec-bundle \
  -input ./internal/agent/research/data/splits/baseline-v1/test.bundle.json \
  -embed-provider ollama \
  -embed-model qwen3-embedding:0.6b \
  -embed-dims 1024 \
  -collection agent_episodic_memory_qwen3_1024
```

### Step 6. Run the Qwen3 retrieval benchmark

```bash
cd /Users/zhanzihao/Lumilio-Photos/server/internal/agent/research
uv run agent-memory-research benchmark-retrieval \
  data/splits/baseline-v1/test.bundle.json \
  --collection agent_episodic_memory_qwen3_1024 \
  --embed-model qwen3-embedding:0.6b \
  --embed-dims 1024 \
  --output-path data/reports/qwen3-baseline-v1.json
```

### Step 7. Seed Granite embeddings into Qdrant

Use a separate collection from Qwen3.

```bash
cd /Users/zhanzihao/Lumilio-Photos/server
go run ./internal/agent seed-spec-bundle \
  -input ./internal/agent/research/data/splits/baseline-v1/test.bundle.json \
  -embed-provider ollama \
  -embed-model granite-embedding:278m \
  -embed-dims 768 \
  -collection agent_episodic_memory_granite_768
```

### Step 8. Run the Granite retrieval benchmark

```bash
cd /Users/zhanzihao/Lumilio-Photos/server/internal/agent/research
uv run agent-memory-research benchmark-retrieval \
  data/splits/baseline-v1/test.bundle.json \
  --collection agent_episodic_memory_granite_768 \
  --embed-model granite-embedding:278m \
  --embed-dims 768 \
  --output-path data/reports/granite-baseline-v1.json
```

## Baseline V1 Result Snapshot

Current report files:

- `data/reports/qwen3-baseline-v1.json`
- `data/reports/granite-baseline-v1.json`

Observed metrics on the current `baseline-v1` test split:

- `Qwen3-Embedding-0.6B @ 1024d`
  - `Recall@1 = 1.0`
  - `MRR@10 = 1.0`
  - `end_to_end p50 ≈ 84ms`
  - `end_to_end p95 ≈ 441ms`
- `Granite-Embedding-278M @ 768d`
  - `Recall@1 = 1.0`
  - `MRR@10 = 1.0`
  - `end_to_end p50 ≈ 89ms`
  - `end_to_end p95 ≈ 141ms`

These numbers should be treated as a sanity check, not a final baseline.

Reason:

- test split is still very small
- test coverage currently lands on one task family
- this confirms the pipeline works, but it does not yet separate model quality reliably

Historical note:

- `baseline-v1` and `baseline-v2` were originally scored with family-level matching
- the harness now supports instance-level matching via `target_episode_ids`
- future baselines should be regenerated and rescored under the new instance-level rule

## Recommended Next Experiment

Before LoRA or MRL, run a stronger baseline:

- `120-200` episodes
- `40-60` queries
- multiple `scenario + intent` groups in the test split
- the same two-model comparison:
  - `qwen3-embedding:0.6b`
  - `granite-embedding:278m`

Only after that should you lock a baseline and start:

- contrastive triplet generation
- LoRA fine-tuning
- MRL dimension sweeps

## Troubleshooting

### DeepSeek returns invalid JSON

Use smaller batch sizes:

```bash
uv run agent-memory-research generate-spec-bundle data/raw/smoke.bundle.json \
  --episode-count 24 \
  --batch-episode-count 6 \
  --max-tokens 5000
```

### Qdrant returns 404 for a benchmark collection

That usually means the collection has not been seeded yet.
Run `seed-spec-bundle` for that model and collection first.

### Ollama works but latency is unstable

This is expected on local models, especially for the first request.
Always compare `p50` and `p95`, not just mean latency.

### `go test ./...` fails outside the agent module

There is a known unrelated failure in `server/internal/utils/errgroup`.
It is not part of this research harness.
