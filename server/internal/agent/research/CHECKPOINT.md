# Episodic Memory Research Checkpoint

Date: 2026-04-01

Checkpoint Scope: `server/internal/agent` and `server/internal/agent/research`

## Research Objective

This checkpoint freezes the first academically meaningful baseline for the media-agent episodic memory study.

The core research question is:

- can a media-management agent retrieve the correct prior episode instance, not just the correct task family, using an embedding-based episodic memory stack?

The current system treats episodic memory as a RAG-like retrieval layer:

- past task executions are written as structured episodes
- episodes are embedded and stored in Qdrant
- a new query retrieves relevant prior episodes
- retrieved episodes are intended to be re-injected into the agent as decision context

## System Boundary

The implementation is intentionally split into three layers:

- Go runtime harness under `server/internal/agent`
- Python research harness under `server/internal/agent/research`
- shared JSON Schema contracts under `server/internal/agent/schemas`

This boundary is now stable enough to support reproducible data generation, retrieval benchmarking, and future training experiments.

## Methodological Progress

### Phase 1: family-level retrieval harness

The initial harness established:

- synthetic media-domain tools and episode writing
- Qdrant-backed retrieval
- Ollama-hosted embedding benchmarks
- DeepSeek-driven dataset generation

At this stage, retrieval was evaluated at the `scenario + intent` level.
This was useful for pipeline sanity checks, but too coarse for research conclusions.

### Phase 2: instance-level evaluation

The second phase introduced explicit instance-level supervision:

- every episode now carries a stable `episode_id`
- every query now carries `target_episode_ids`
- benchmark hit logic now requires the retrieved episode ID to match one of the declared target IDs

This change converted the benchmark from task-family retrieval into episode-instance retrieval.
As expected, metrics dropped substantially relative to family-level evaluation, which confirmed that the benchmark had become meaningfully harder.

### Phase 3: minimal-difference clusters and balanced generation planning

The current checkpoint adds cluster-aware data design:

- every episode now carries `cluster_id`
- generation is driven by a programmatic generation matrix before DeepSeek is called
- each cluster contains minimal-difference variants around an anchor episode
- train/val/test splits are now stratified by `scenario + cluster_id`

This is the first checkpoint where the test split is deliberately shaped to contain:

- multiple task families
- multiple cluster instances per task family
- near-neighbor distractors that stress instance discrimination

## Current Data Contract

The shared bundle contract now includes:

- `episode_id`
- `cluster_id`
- `target_episode_ids`

These fields are the minimum required to support:

- instance-level retrieval benchmarking
- hard-negative analysis
- future triplet construction for contrastive training

## Current Generation and Evaluation Pipeline

### Data generation

DeepSeek is no longer asked to generate free-form batches with only weak guidance.
Instead:

1. Python builds a deterministic generation matrix
2. each batch is sliced from that global plan
3. DeepSeek fills the content for the batch
4. the result is normalized, schema-validated, and linted
5. missing query coverage is backfilled if necessary

This reduces uncontrolled drift and substantially improves reproducibility.

### Split construction

`make-splits` now operates on `scenario + cluster_id`, not only on `scenario + intent`.
This prevents the test split from collapsing into a small number of task families.

### Retrieval benchmark

Benchmarking is currently run against two local embedding baselines:

- `qwen3-embedding:0.6b @ 1024d`
- `granite-embedding:278m @ 768d`

The benchmark reports:

- `Recall@1`
- `Recall@5`
- `Recall@10`
- `MRR@10`
- embedding latency
- vector search latency
- end-to-end latency

## Baseline Evolution

### V1

Interpretation:

- pipeline sanity check only
- test split too small
- only one task family in test

### V2

Interpretation:

- family-level benchmark was stable
- still optimistic
- not suitable for model comparison at instance granularity

### V3

Interpretation:

- first valid instance-level baseline
- metrics dropped in the expected way
- revealed real confusion between near-identical episode instances
- test distribution still skewed

### V4

Interpretation:

- first balanced instance-level baseline suitable for freezing
- test split covers all seven task groups
- each task group contributes multiple clusters
- benchmark now reflects both family-level semantic retrieval and intra-family episode discrimination

## Baseline V4 Dataset Summary

Source:

- `data/raw/baseline-v4.bundle.json`

Split manifest:

- `data/splits/baseline-v4/manifest.json`

Counts:

- episodes: `180`
- queries: `72`
- train episodes: `99`
- val episodes: `21`
- test episodes: `60`

Test split properties:

- all `7` scenario-intent groups are represented in episodes
- all `7` scenario-intent groups are represented in queries
- each test scenario currently has `3` clusters

This is the first test split in the project that is broad enough to serve as a serious pre-training baseline.

## Baseline V4 Retrieval Results

Report files:

- `data/reports/qwen3-baseline-v4.json`
- `data/reports/granite-baseline-v4.json`

### Granite

- `Recall@1 = 0.592593`
- `Recall@5 = 0.962963`
- `Recall@10 = 0.962963`
- `MRR@10 = 0.771605`
- `end_to_end p50 ≈ 91.855 ms`
- `end_to_end p95 ≈ 113.131 ms`

### Qwen3

- `Recall@1 = 0.481481`
- `Recall@5 = 0.925926`
- `Recall@10 = 0.962963`
- `MRR@10 = 0.674074`
- `end_to_end p50 ≈ 69.007 ms`
- `end_to_end p95 ≈ 85.593 ms`

### Interpretation

The main empirical finding at this checkpoint is:

- `Granite` is the stronger retrieval baseline on instance-level accuracy
- `Qwen3` is the faster latency baseline on the current local setup

This gives the project a useful pre-training frontier:

- retrieval quality anchor: `Granite`
- latency anchor: `Qwen3`

## Error Analysis

The remaining failures are now informative rather than pathological.

Most top-1 misses fall into one of these categories:

- the model retrieves a different anchor from the same task family and nearly identical entity bundle
- the model prefers a minimal-difference variant over the correct anchor
- the model confuses episodes that differ only along a small number of salient fields such as:
  - location
  - time window
  - camera model
  - rating threshold
  - failure mode

This is exactly the failure surface needed for contrastive fine-tuning.

## What This Checkpoint Establishes

This checkpoint establishes the following research baseline:

- the episodic memory harness is operational end-to-end
- the data contract is stable enough for controlled experiments
- the benchmark now measures episode-instance retrieval rather than family-only retrieval
- the test distribution is sufficiently broad for meaningful baseline comparison
- the current failure modes are appropriate for hard-negative training

## Remaining Limitations

The current baseline is strong enough to train against, but still has limits:

- the data is synthetic rather than traced from a real user population
- some scenario families still have limited linguistic diversity relative to real traffic
- the validation split is smaller and less query-balanced than the test split
- the generation process still depends on LLM content fidelity, even though distribution is now program-controlled

## Next Phase

The next research phase should move from baseline construction to training and ablation.

Planned work:

1. construct explicit contrastive triplets from cluster-aware episodes
2. train LoRA adapters for at least one embedding baseline
3. compare pre-training and post-training retrieval on the frozen `baseline-v4` benchmark
4. run MRL-style dimensional sweeps to study quality-latency tradeoffs
5. add a small naturalistic evaluation set as a sanity check against overfitting to the controlled matrix

## Checkpoint Decision

This checkpoint should be treated as:

- the final frozen untrained baseline for the current study
- the reference point for all subsequent LoRA, contrastive, and MRL experiments

From this point onward, new experimental results should be reported relative to `baseline-v4`.
