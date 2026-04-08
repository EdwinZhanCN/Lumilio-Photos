# Strict Retrieval Review

## Issue 1: `target_episode_ids` is treated as a positive set, not a candidate set

Current `baseline-v4` data and evaluation do not encode a strict "single positive episode" contract.

The core mismatch is:

- In the dataset shape, `query.target_episode_ids` may look like "episodes related to this query".
- In the evaluator, `query.target_episode_ids` is actually used as the ground-truth positive set: if retrieval hits any ID in that array, the query is counted as correct.

This means that once a query contains more than one `target_episode_id`, the benchmark is no longer strict single-positive retrieval.

### Evidence

Schema only requires at least one target episode ID and does not cap the array at one item:

- [`server/internal/agent/schemas/episode_spec_bundle.schema.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/schemas/episode_spec_bundle.schema.json): `querySpec.target_episode_ids` has `minItems: 1` but no `maxItems: 1`

Prompt text also permits multiple targets:

- [`server/internal/agent/research/src/agent_memory_research/deepseek_client.py`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/src/agent_memory_research/deepseek_client.py): "one or a very small number of exact episode_id values"

Evaluator semantics treat the field as a positive set:

- [`server/internal/agent/research/src/agent_memory_research/cli.py`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/src/agent_memory_research/cli.py): `match_rank(...)` returns success if any retrieved episode ID is contained in `target_episode_ids`

There is already a committed `baseline-v4` example with two target episode IDs:

- [`server/internal/agent/research/data/splits/baseline-v4/test.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v4/test.bundle.json): query `"What camera settings were used for the Yosemite spring 2024 photos taken with the Canon R5?"`

### Why this matters

For strict retrieval, a query must have exactly one semantic positive episode.

If `target_episode_ids` contains multiple acceptable answers, then:

- the benchmark becomes multi-positive retrieval
- Recall@K and MRR are computed against a weaker target condition
- the data contract no longer enforces the "unique positive" principle

### Current conclusion

This issue should be tracked as:

- not a naming problem alone
- a contract plus evaluator semantics problem

Even if the intended meaning was "candidate episodes", the current implementation uses the field as "acceptable positives".

### Follow-up options

Two valid fixes exist:

1. Keep the schema and add strict lint that enforces `len(target_episode_ids) == 1` for strict datasets.
2. Tighten the schema for strict datasets so the single-positive constraint is part of the data contract.

For now, the important recorded point is:

- `baseline-v4` does not currently guarantee strict single-positive semantics at the data-contract level.

## Issue 2: query generation prompt does not enforce discriminative slots

To satisfy strict retrieval, a query must include enough discriminative slots to isolate one target episode from nearby alternatives.

The current prompt does not enforce that requirement.

### Prompt-design gap

The generation plan already exposes useful structure to the model:

- `query_blueprints` includes `entity_bundle`
- `query_blueprints` includes `minimal_difference_axis`
- the full `episode_blueprints` matrix is also passed into the prompt

Relevant sources:

- [`server/internal/agent/research/src/agent_memory_research/generation_matrix.py`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/src/agent_memory_research/generation_matrix.py): query blueprints carry `entity_bundle` and `minimal_difference_axis`
- [`server/internal/agent/research/src/agent_memory_research/generation_matrix.py`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/src/agent_memory_research/generation_matrix.py): `query_focus` is only a broad family/topic hint such as `"duplicate cleanup"` or `"album creation"`

But the actual prompt only asks for:

- targeting a specific prior episode
- paraphrasing the goal
- following the generation plan

Relevant source:

- [`server/internal/agent/research/src/agent_memory_research/deepseek_client.py`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/src/agent_memory_research/deepseek_client.py)

What is missing is an explicit instruction like:

- include the slots needed to distinguish the target from other same-scenario or same-intent episodes
- if `minimal_difference_axis` is not `baseline`, explicitly mention that axis in the query
- rewrite the query until exactly one episode in the batch remains compatible

Because that instruction is absent, the model can satisfy the current prompt with broad paraphrases that still sound episode-specific to a human, while remaining ambiguous against nearby variants.

### Evidence from committed `baseline-v4` samples

The clearest evidence is the set of queries whose targets are near variants.

In the committed test split, all three such queries omit the very slot that differentiates the target from its nearby alternative:

1. Near `time_window` target:
   - target episode: [`server/internal/agent/research/data/splits/baseline-v4/test.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v4/test.bundle.json) `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c01_near_time_window_sony_a7c_ii_false_positive_duplicate_yosemite_0_82_spring_2023`
   - query: [`server/internal/agent/research/data/splits/baseline-v4/test.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v4/test.bundle.json) `"How did we handle duplicate photos from the Yosemite shoot with Sony camera?"`
   - missing discriminative slot: `spring 2023`

2. Near `failure_mode` target:
   - target episode: [`server/internal/agent/research/data/splits/baseline-v4/test.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v4/test.bundle.json) `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c09_near_failure_mode_sony_a7c_ii_near_duplicate_burst_yosemite_0_82`
   - query: [`server/internal/agent/research/data/splits/baseline-v4/test.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v4/test.bundle.json) `"How did I handle removing duplicate photos from Yosemite shot with my Sony camera?"`
   - missing discriminative slot: `near-duplicate burst`

3. Near `rating_threshold` target:
   - target episode: `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c03_near_rating_threshold_canon_eos_r5_near_duplicate_burst_portrait_session_3_0_76`
   - query: `"What happened when I tried to clean up duplicate portrait session photos?"`
   - missing discriminative slot: `>=3` / rating threshold

This is strong evidence that the prompt is not turning `minimal_difference_axis` into a lexical obligation.

### Why this is a prompt problem first

This is not only a post-generation lint problem.

The generation stack already has the raw ingredients needed for discriminative queries:

- target entity bundle
- cluster identity
- minimal-difference axis
- full batch episode matrix

So the bottleneck is the instruction policy:

- the prompt rewards paraphrase
- the prompt does not require contrast against nearest neighbors
- the prompt does not require explicit mention of differentiating slots

As a result, generated queries drift toward family-level naturalness instead of strict instance-level distinguishability.

### Prompt changes that would directly address this

If this is solved at the prompt layer, the prompt should require all of the following:

1. A query must contain enough slots to identify exactly one compatible episode among all episodes in the supplied batch.
2. The model must compare the target episode against same-scenario and same-intent neighbors before writing the query.
3. If `minimal_difference_axis != "baseline"`, the query must explicitly express that axis.
4. If a slot is required to separate the target from a nearby anchor or sibling variant, that slot must appear in the query text.
5. After drafting the query, the model must run a self-check: if more than one episode in the batch could satisfy the query, rewrite it to add discriminative slots.

### Current conclusion

Issue 2 should be tracked as:

- a prompt-design failure to enforce discriminative slot coverage
- not merely a wording-quality issue

Under the current prompt, broad paraphrases are still rewarded even when they are not strict-retrieval-safe.

## Issue 3: retrieval text often expresses core slots, but expressive completeness is not guaranteed

The current retrieval-text builder is:

- stronger than Issue 2 might suggest
- but still weaker than a true strict-retrieval contract

### What the retrieval text currently contains

`BuildRetrievalText()` serializes:

- `goal`
- `intent`
- `scenario`
- `summary`
- `entities`
- `tags`
- tool names
- `status`

Relevant source:

- [`server/internal/agent/memory/schema.go`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/memory/schema.go)

When synthetic spec episodes are materialized, metadata is preserved on the episode object, but retrieval text is still built only from the fields above:

- [`server/internal/agent/memory/synthetic/spec.go`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/memory/synthetic/spec.go)

Qdrant stores the full episode payload, including metadata inside `episode`, but embeddings are computed from `retrieval_text`, not from arbitrary payload fields:

- [`server/internal/agent/memory/qdrant.go`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/memory/qdrant.go)

### Important nuance: many hard samples are already expressible

For several of the hard samples from Issue 2, the target retrieval text actually does contain the missing discriminative slot.

Examples from the committed test split:

1. Near `time_window` target:
   - episode: [`server/internal/agent/research/data/splits/baseline-v4/test.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v4/test.bundle.json) `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c01_near_time_window_sony_a7c_ii_false_positive_duplicate_yosemite_0_82_spring_2023`
   - retrieval text would include goal/summary/entities with `spring 2023`

2. Near `failure_mode` target:
   - episode: [`server/internal/agent/research/data/splits/baseline-v4/test.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v4/test.bundle.json) `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c09_near_failure_mode_sony_a7c_ii_near_duplicate_burst_yosemite_0_82`
   - retrieval text would include `near-duplicate burst shots`

3. Near `rating_threshold` target:
   - episode: [`server/internal/agent/research/data/splits/baseline-v4/test.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/splits/baseline-v4/test.bundle.json) `ep_cleanup_duplicate_shoot_cleanup_duplicate_shoot_c03_near_rating_threshold_canon_eos_r5_near_duplicate_burst_portrait_session_3_0_76`
   - retrieval text would include `highest rated (≥3)` and `rating_threshold=3`

So for those cases, the main failure is:

- query under-specification

not:

- target retrieval text missing the slot entirely

### The real builder-level gap

The builder does **not** guarantee that every discriminative slot ends up in retrieval text.

Any slot that exists only in:

- `metadata`
- step inputs
- step payloads
- step output summaries
- context blocks

will be absent from the embedding text unless it is redundantly copied into goal, summary, entities, or tags.

That means expressive completeness is currently accidental, not enforced.

### Concrete examples of the gap

#### 1. Metadata-only meaningful slot

This raw `baseline-v4` episode stores `liked_state=false` only in metadata:

- [`server/internal/agent/research/data/raw/baseline-v4.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/raw/baseline-v4.bundle.json) `ep_bulk_like_highlights_bulk_like_highlights_c02_near_time_window_paris_favorites_false_paris_5_autumn_2024`

Its visible retrieval-text ingredients are:

- goal: `"Like all high-rated Paris photos from autumn 2024"`
- summary: `"Successfully liked 42 Paris photos with rating 5 from autumn 2024"`
- entities: `Paris`, `Paris Favorites`, `autumn_2024`

But the metadata also contains:

- `liked_state = false`

That slot is not explicitly surfaced in retrieval text for this episode.

So if a strict query needed to distinguish:

- `unliked 5-star Paris photos`

from:

- other high-rated Paris-photo episodes

the current retrieval text would not guarantee that distinction.

#### 2. Dataset-level expressibility failure caused by wrong target assignment

There is also a committed raw `baseline-v4` example where the query asks for `spring 2024`, but the assigned target episode retrieval text says `spring 2023`:

- query: [`server/internal/agent/research/data/raw/baseline-v4.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/raw/baseline-v4.bundle.json) `"How did we check camera details for Canon EOS R5 shots in Yosemite during spring 2024?"`
- target episode: [`server/internal/agent/research/data/raw/baseline-v4.bundle.json`](/Users/zhanzihao/Lumilio-Photos/server/internal/agent/research/data/raw/baseline-v4.bundle.json) `ep_inspect_camera_metadata_inspect_camera_metadata_c01_near_time_window_canon_eos_r5_yosemite_spring_2023`

This is partly a wrong-label problem, but it also proves the current pipeline does not preserve the expressibility invariant end to end.

### Current conclusion

Issue 3 should be stated carefully:

- `baseline-v4` retrieval text is **often** expressive enough for common core slots
- the current retrieval-text builder does **not** guarantee expressive completeness

So the principle is:

- partially satisfied at the sample level
- not satisfied at the contract/construction level

### What would make this robust

To satisfy strict retrieval reliably, retrieval text should be generated from an explicit slot projection rather than relying on free-form goal/summary wording.

At minimum, the embedding text should include normalized slot lines for the fields that matter for discrimination, for example:

- `location=...`
- `camera_model=...`
- `time_window=...`
- `rating_threshold=...`
- `failure_mode=...`
- `liked_state=...`
- `group_by=...`
- `selection_theme=...`

That would turn expressibility from an incidental property into a guaranteed one.

## Remediation Plan

This plan treats the current problems in priority order:

1. stop label drift
2. enforce single-positive semantics
3. enforce discriminative query generation
4. guarantee retrieval-text expressibility
5. rebuild and re-benchmark the dataset

### Phase 1: stop target rewrite

Goal:

- ensure the final `target_episode_ids` written to disk stays aligned with the program-selected target from the generation plan

Changes:

- remove or disable `force_retarget_queries=True` in the raw bundle generation path
- stop using `resolve_query_target_episode_ids()` as a post-generation relabeling step for strict datasets
- keep normalization for ID cleanup and metadata stringification only

Acceptance criteria:

- for a regenerated bundle, the committed query targets match the original query blueprints
- no query target changes after normalization

Priority:

- highest

Reason:

- all downstream strictness work is invalid if the final labels can still drift after generation

### Phase 2: enforce single-positive semantics

Goal:

- a strict query must resolve to exactly one target episode

Changes:

- add strict lint that requires exactly one `target_episode_id` for strict datasets
- add a strict bundle validation mode that rejects multi-target queries
- optionally add a strict schema or strict query field later, but do not block on schema redesign first

Acceptance criteria:

- `len(target_episode_ids) == 1` for every strict query
- strict validation fails immediately on any multi-target query

Priority:

- high

Reason:

- this is the minimum structural requirement for the "unique positive" principle

### Phase 3: make the prompt require discriminative slots

Goal:

- generated query text must explicitly carry enough slot information to distinguish the target from nearby alternatives

Changes:

- extend `query_blueprints` with explicit discriminative guidance, at minimum:
  - `required_slots`
  - `minimal_difference_axis`
  - optional `hard_negative_episode_ids`
- change the DeepSeek prompt so that:
  - the query must include the `required_slots`
  - if `minimal_difference_axis != baseline`, that axis must be explicitly expressed
  - the model must rewrite the query if more than one same-scope episode could satisfy it

Acceptance criteria:

- regenerated strict queries mention the slots that separate the target from same-scenario or same-intent neighbors
- near-variant targets are no longer paired with broad family-level paraphrases

Priority:

- high

Reason:

- the current prompt produces natural queries, but does not reliably produce uniquely identifying queries

### Phase 4: guarantee retrieval-text expressibility

Goal:

- every slot required by a strict query must be explicitly available in the target embedding text

Changes:

- add a normalized slot projection to `retrieval_text`
- do not rely only on free-form `goal` and `summary`
- include discriminative slot lines for fields such as:
  - `location`
  - `camera_model`
  - `time_window`
  - `rating_threshold`
  - `failure_mode`
  - `liked_state`
  - `group_by`
  - `selection_theme`

Acceptance criteria:

- for every strict query, all required discriminative slots appear in the target retrieval text
- no strict query depends on a metadata-only slot that is absent from embedding text

Priority:

- medium

Reason:

- current samples often work, but builder-level guarantees are missing

### Phase 5: add strict validation

Goal:

- make strict retrieval quality a programmatic pass/fail check, not a manual review task

Changes:

- add a strict linter that checks:
  - unique positive
  - discriminative slot coverage
  - retrieval-text expressibility
  - no post-normalization target drift
- run this linter as part of bundle validation before a dataset is accepted

Acceptance criteria:

- a bundle cannot be published as strict if it violates any of the four rules above

Priority:

- medium

Reason:

- prompt changes alone are not stable enough without deterministic enforcement

### Phase 6: regenerate and freeze a new strict baseline

Goal:

- replace the current `baseline-v4` as a strict benchmark with a regenerated validated version

Changes:

- regenerate raw bundle after phases 1 to 5 land
- recreate train/val/test splits
- reseed retrieval collections
- rerun benchmark reports
- freeze the new bundle and reports together

Acceptance criteria:

- bundle, splits, and reports are internally consistent
- no query/report target mismatch
- strict linter passes on raw and split bundles

Priority:

- final

Reason:

- the current frozen artifact cannot serve as a strict baseline until the generation and validation path is fixed

## Practical Order

Recommended implementation order:

1. remove target rewriting
2. add strict lint for single-positive queries
3. extend query blueprints with discriminative slot requirements
4. tighten the prompt
5. extend retrieval text with normalized slot projection
6. add full strict validation
7. regenerate the dataset and rerun benchmarks

## Non-Goal for the first pass

Do not try to redesign the whole research CLI first.

For the first strict-retrieval pass, it is enough to:

- stop target drift
- make query generation stricter
- make retrieval text slot-complete
- add deterministic validation

The CLI and generation matrix can be simplified later after correctness is restored.
