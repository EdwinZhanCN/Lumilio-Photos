# Episodic Memory Research Checkpoint

Date: 2026-03-30

Commit: `c7886a4`

## Scope

This checkpoint freezes the first end-to-end episodic memory research harness for the media-management agent.

Included in this checkpoint:

- Go runtime seeding and search harness under `server/internal/agent`
- Python research CLI under `server/internal/agent/research`
- shared JSON Schema contracts under `server/internal/agent/schemas`
- synthetic media-domain datasets
- Qdrant-backed retrieval benchmarking
- baseline comparisons for `qwen3-embedding:0.6b` and `granite-embedding:278m`

## Implemented Components

### Runtime side

- `main.go` supports:
  - `list-tools`
  - `print-spec-example`
  - `seed-spec-bundle`
  - `seed-episodes`
  - `search`
- `memory/` contains:
  - Qdrant store
  - hash and Ollama embedders
  - episode schema
  - writer
  - synthetic media episode compiler
- `mock_tools/` provides media-management mock tools only

### Research side

- `agent-memory-research generate-spec-bundle`
- `agent-memory-research validate-bundle`
- `agent-memory-research make-splits`
- `agent-memory-research benchmark-retrieval`

### Shared contracts

- `episode_spec_bundle.schema.json`
- `dataset_manifest.schema.json`
- `retrieval_benchmark_report.schema.json`

## Current Data Generation Rules

- DeepSeek generates synthetic media-management episodes and retrieval queries in batches.
- batch outputs are normalized before schema validation
- numeric metadata values are coerced to strings
- missing query coverage for any `scenario + intent` group is backfilled automatically
- benchmark filters use exact constraints for `entity` and `status`
- `tags` are not used as hard filters

## Baseline Artifacts

Generated bundles:

- `data/raw/smoke.bundle.json`
- `data/raw/baseline-smoke.bundle.json`
- `data/raw/baseline-v1.bundle.json`
- `data/raw/baseline-v2.bundle.json`

Current benchmark reports:

- `data/reports/qwen3-baseline-v1.json`
- `data/reports/granite-baseline-v1.json`
- `data/reports/qwen3-baseline-v2.json`
- `data/reports/granite-baseline-v2.json`

## Baseline V1 Summary

Dataset shape:

- episodes: 35
- queries: 18
- test episodes: 5
- test queries: 2

Result:

- `Qwen3-Embedding-0.6B @ 1024d`
  - `Recall@1 = 1.0`
  - `MRR@10 = 1.0`
  - `end_to_end p50 ~= 84ms`
  - `end_to_end p95 ~= 441ms`
- `Granite-Embedding-278M @ 768d`
  - `Recall@1 = 1.0`
  - `MRR@10 = 1.0`
  - `end_to_end p50 ~= 89ms`
  - `end_to_end p95 ~= 141ms`

Interpretation:

- valid sanity check
- too small for model comparison
- only one task family in test split

## Baseline V2 Summary

Dataset shape:

- episodes: 140
- queries: 56
- test episodes: 28
- test queries: 13

Observed test split coverage:

- `inspect_camera_metadata / inspect_metadata`
- `archive_low_rated_assets / bulk_archive`

Reported metrics:

- `Qwen3-Embedding-0.6B @ 1024d`
  - `Recall@1 = 1.0`
  - `MRR@10 = 1.0`
  - `end_to_end p50 ~= 74ms`
  - `end_to_end p95 ~= 84ms`
- `Granite-Embedding-278M @ 768d`
  - `Recall@1 = 1.0`
  - `MRR@10 = 1.0`
  - `end_to_end p50 ~= 92ms`
  - `end_to_end p95 ~= 98ms`

Interpretation:

- runtime and evaluation pipeline are stable
- both models reliably retrieve the correct task family
- Qwen3 is currently faster on this local setup
- metric values are still optimistic because hit logic is family-level, not instance-level

## Known Evaluation Limitation

Current benchmark hit logic is too coarse.

`match_rank()` counts a retrieval as correct when the returned episode matches:

- `scenario`
- `intent`

It does not require matching a specific historical episode instance.

Practical consequence:

- if a query about one Canon metadata episode retrieves a different Canon metadata episode from the same family, it is still counted as a hit
- this inflates `Recall@1` and `MRR`
- it prevents reliable measurement of pattern separation

## Decision At This Checkpoint

The next phase should move from family-level retrieval to instance-level retrieval.

Planned changes:

- add `target_episode_ids` to query-level ground truth
- update benchmark matching to require retrieved episode IDs to be in `target_episode_ids`
- generate minimal-difference hard-negative clusters
- measure whether baseline models confuse near-identical episodes
- only then start LoRA, contrastive training, and MRL sweeps

## Immediate Next Step

Implement instance-level evaluation:

1. extend `QuerySpec` and the schema with `target_episode_ids`
2. regenerate datasets with explicit episode-level supervision
3. update `benchmark-retrieval` to score against episode IDs
4. rerun baseline before any training

## Notes

- this checkpoint is intended to be a stable before/after comparison point
- no attempt has been made yet to optimize prompts or training data for LoRA
- there is a known unrelated repository test failure under `server/internal/utils/errgroup`; it is outside this research harness
