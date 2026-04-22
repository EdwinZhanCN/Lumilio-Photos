# Benchmark Problem Statement for Episodic Memory Retrieval

This note records the concrete problems in the current episodic memory retrieval benchmark before selecting related papers or designing LoRA experiments.

The goal is to separate three different questions that are currently entangled:

1. Is the retriever actually good at episode-level semantic recall?
2. Is the benchmark accidentally easy because of query construction and dataset structure?
3. If we apply LoRA, will it improve real episodic recall or only learn to match templates and explicit slots?

## Current Diagnosis

The current benchmark appears to overestimate retrieval quality.

The main issue is not an obvious train/test split leak such as `cluster_id` entering the embedding text.
The larger issue is that the benchmark has become too easy to solve through explicit slot matching, lexical overlap, and templatic query generation.

As a result, current strong scores do not yet justify the conclusion that the memory formulation is already strong enough, nor do they provide a reliable basis for judging whether LoRA is useful.

## Problem 1: Query Construction Leakage

The query generation pipeline currently gives the query writer direct access to target-discriminating information.

Relevant code:

- `server/internal/agent/research/src/agent_memory_research/generation_matrix.py`
- `server/internal/agent/research/src/agent_memory_research/deepseek_client.py`

Observed behavior:

- query blueprints are generated only for strict targets selected from anchor episodes
- each query blueprint carries `required_slots`
- each query blueprint also carries `hard_negative_episode_ids`
- the DeepSeek prompt explicitly tells the model to express every required slot in the query
- the prompt further tells the model to rewrite until the query can only match the target episode

Why this is a problem:

- this turns query generation into a target-aware discrimination exercise
- the benchmark no longer measures natural recall from partial or noisy user cues
- the resulting query is closer to a synthetic retrieval key than a realistic episodic recall request

Implication:

The benchmark likely measures whether the retriever can recover an episode from its explicitly exposed differentiating fields, not whether it can recall the right episode from weak natural-language cues.

## Problem 2: Synthetic Backfill Queries Inflate Test Performance

The split pipeline currently backfills missing test coverage by synthesizing queries directly from the target episode.

Relevant code:

- `server/internal/agent/research/src/agent_memory_research/cli.py`

Observed behavior:

- `make-splits` can enforce one query per test episode
- missing queries are added by `backfill_queries_for_episodes`
- `synthesize_query_from_episode` often generates:
  - `How did I handle {goal} for {entity} last time?`

Observed dataset effect:

- in `baseline-v6-large-test`, 35 of 59 test queries are synthetic coverage backfills

Why this is a problem:

- these queries are created from the target episode's own goal and entities
- they often restate target information nearly directly
- they are much easier than naturally authored recall queries

Observed evaluation effect:

- on `qwen3-baseline-v6-large-semantic.json`
  - overall `recall@1 = 0.728814`
  - backfill subset `recall@1 = 0.857`
  - non-backfill subset `recall@1 = 0.542`

Implication:

The reported benchmark score is materially lifted by synthetic test queries that should not be treated as equivalent to realistic recall prompts.

## Problem 3: The Benchmark Is Strongly Lexically Solvable

The current retrieval setting can often be solved by crude lexical overlap rather than robust semantic episode recall.

Relevant retrieval text construction:

- `server/internal/agent/memory/schema.go`

Current dense retrieval text includes:

- `scenario`
- `intent`
- `summary`
- `goal`
- `task_content`
- `procedure`

Observed query pattern:

- many test queries explicitly include location, camera model, album name, time window, rating threshold, or similarity threshold
- many synthetic queries directly reuse target goal phrasing

Empirical finding:

- a simple token-overlap baseline over episode text is already extremely strong
- on `baseline-v6`, a crude lexical baseline reached perfect `recall@1`
- on `baseline-v6-large-test`, the same style of lexical scoring remained extremely high

Why this is a problem:

- if a weak lexical baseline already solves the task, then high dense retrieval scores do not prove strong semantic episodic recall
- a later LoRA may only learn better template alignment or slot overlap

Implication:

Before training LoRA, we need a benchmark where lexical matching is no longer enough.

## Problem 4: Candidate Set Difficulty Is Still Too Low

The test benchmark is still relatively easy at the neighborhood level.

Observed structure in `baseline-v6-large-test`:

- total test episodes: 56
- total test queries: 59
- each scenario has only about 3 clusters
- for a given query, the same-scenario candidate set is often only 6 to 9 episodes

Observed behavior:

- the model almost always gets the correct scenario family
- the remaining task is often reduced to picking the explicit slot combination inside that small scenario-local set

Why this is a problem:

- the benchmark acts partly like scenario classification plus slot disambiguation
- that is easier than full corpus-level episode retrieval from under-specified cues

Implication:

We still need harder same-family negatives and larger within-scenario ambiguity.

## Problem 5: Tool Trace Has Limited Discriminative Power in the Current Dataset

The design document treats tool trace as an important agent-specific memory cue, but the current dataset does not fully stress that dimension.

Observed pattern:

- within most scenario/intent groups, tool traces are nearly fixed
- only one scenario family currently shows meaningful procedural variation

Why this is a problem:

- the benchmark does not really test whether the retriever uses procedure as a recall cue
- `Tool Trace` is conceptually important in the design, but weakly exercised in evaluation

Implication:

If future LoRA gains are small, that may simply mean the benchmark is not yet asking the model to use procedural memory.

## Problem 6: Query Distribution Is Not Yet Realistic for Episodic Recall

A real user often remembers only fragments of a prior experience.

Typical real recall queries are more like:

- the duplicate cleanup that had false positives
- the case where metadata looked wrong but we did not archive
- the trip album we built from the Fuji shots

The current dataset more often uses queries that fully specify:

- task family
- place
- time window
- device
- threshold
- album

Why this is a problem:

- the benchmark favors explicit content matching over memory-like reconstruction
- it does not sufficiently test recall from partial, noisy, or indirect cues

Implication:

We need a harder query regime where only one or two cues are present, and the target episode must be recovered from latent semantic and procedural structure.

## Problem 7: LoRA Value Is Currently Unidentifiable

Because the benchmark is easy in the wrong ways, a future LoRA experiment would be hard to interpret.

Possible outcomes:

- if LoRA shows little gain, the benchmark may already be saturated by lexical shortcuts
- if LoRA shows gain, it may just be learning slot-template alignment instead of real episodic discrimination

Why this matters:

At the moment, the benchmark does not cleanly answer whether LoRA improves:

- semantic recall
- hard negative discrimination
- partial-cue recall
- procedural recall

Implication:

The benchmark must be hardened before LoRA results can be considered meaningful.

## What This Means for Paper Selection

The next papers should not be chosen broadly under the label of "agent memory".

They should be chosen to address the exact benchmark problems above:

1. hard negative construction for dense retrieval
2. contrastive training for instance discrimination
3. query-side reformulation under incomplete cues
4. retrieval evaluation and leakage-resistant benchmark design
5. memory architectures that explicitly model long-term and episodic recall

## Immediate Research Questions

These are the questions that the next literature review should answer:

1. How do dense retrieval papers construct hard negatives that are close but not trivial?
2. How do embedding papers prevent gains from coming only from lexical overlap?
3. How do memory papers define the memory unit and retrieval key for long-horizon agent interaction?
4. What evaluation setup best separates genuine semantic recall from target-aware query leakage?
5. What kind of training data would make LoRA learn episode discrimination rather than slot copying?

## Working Conclusion

The current benchmark is not useless.
It already captures an important direction:

- episode retrieval rather than only task-family classification
- context versus content separation
- explicit goal and tool trace modeling

However, its current form still over-rewards:

- target-aware slot exposure
- synthetic query backfill
- lexical overlap
- small candidate neighborhoods

Therefore the immediate priority is:

1. formalize the benchmark problems
2. read papers that directly address those problems
3. redesign the benchmark before treating LoRA as the main experimental step
