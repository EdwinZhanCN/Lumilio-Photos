# Experiment Design: Precise Retrieval and Reduced-Slot Retrieval

This note defines a practical experiment design for the course project.

The key idea is:

- evaluate two retrieval settings
- but train LoRA for only one of them

This keeps the benchmark richer without making the fine-tuning story too large or too difficult to interpret.

## 1. Motivation

The project currently uses queries that may contain discriminative slots such as location, camera model, album name, time window, rating threshold, or failure mode.

This design is valid for an agent-oriented episodic memory tool because the agent may already know part of the current runtime context. In that setting, the memory system should use the available cues to recover the correct prior episode.

However, it is also useful to test a harder setting in which the query contains fewer cues. That setting can reveal whether the retriever generalizes beyond direct slot matching.

The challenge is that these two settings should not both become LoRA training targets at the same time. If they are mixed into one fine-tuning objective, the experiment becomes harder to interpret and harder to finish within a course-project scope.

For that reason, this design separates:

- **benchmark scope**
- **LoRA training scope**

## 2. Two Retrieval Tasks

### Task A: Precise Retrieval

This is the main task of the project.

Definition:

- the query may contain several discriminative cues
- the retriever must recover the exact target episode
- the main challenge is fine-grained discrimination among similar episodes

Examples of cues:

- location
- album name
- camera model
- time window
- threshold
- failure mode

Interpretation:

- this task should be framed as `agent-guided precise episodic retrieval`
- it is not intended to model fully ambiguous human-style recollection

Why it is the main task:

- it best matches the current project design
- it is easier to justify from an agent systems perspective
- it gives a clean target for LoRA

### Task B: Reduced-Slot Retrieval

This is a secondary task.

Definition:

- the query contains fewer explicit discriminative cues
- the retriever must still recover the target episode
- the task is harder because the query is more underspecified

Examples:

- only one or two slots may be present
- a query may mention only the failure type and not the threshold
- a query may mention only the album or location and not the camera model

Interpretation:

- this task tests partial-cue recall
- it is closer to weakly specified episodic retrieval
- it should be treated as an auxiliary benchmark rather than the main training target

Why it is useful:

- it helps reveal whether the retriever is only performing direct slot matching
- it provides a stronger generalization test

## 3. Why LoRA Should Focus Only on Task A

LoRA should be trained only for the main task: Precise Retrieval.

The reason is not that Task B is unimportant.
The reason is that Task A and Task B emphasize different retrieval behaviors.

Task A emphasizes:

- precise discrimination among near-neighbor episodes
- matching known runtime cues to the correct prior episode

Task B emphasizes:

- robustness under incomplete cues
- inference from weaker semantic signals

If both tasks are mixed into one LoRA training objective, several problems appear:

1. the training distribution becomes less coherent
2. the source of any improvement becomes harder to explain
3. the project becomes larger than necessary for a course setting

Therefore the recommended strategy is:

- train LoRA on Task A only
- evaluate transfer to Task B

This gives a much cleaner experimental story.

## 4. Core Hypothesis

The core hypothesis of the project should be:

> LoRA fine-tuning on hard negatives from the precise retrieval setting improves fine-grained episode discrimination, especially within the same scenario and among minimally different clusters.

This is a narrow and testable claim.

It is much easier to defend than a broader claim such as:

- LoRA improves all episodic retrieval
- LoRA enables human-like memory recall
- LoRA solves ambiguous episodic memory for agents

## 5. Benchmark Structure

The benchmark should be organized into the following parts.

### Main Benchmark

- name: `Precise Retrieval`
- query style: slot-guided
- purpose: evaluate agent-guided exact episode recall

### Auxiliary Benchmark

- name: `Reduced-Slot Retrieval`
- query style: limited-cue
- purpose: evaluate transfer to weaker or less explicit recall prompts

### Optional Debug Set

- name: `Backfill / Coverage Queries`
- query style: synthetic episode-derived queries
- purpose: debugging and sanity checking only

These should not all be merged into a single headline score.

## 6. Query Construction Rules

### For Task A: Precise Retrieval

Allowed:

- include multiple important slots
- include enough information to identify the target episode
- reflect realistic agent-side runtime cues

Not recommended:

- direct template backfill from `goal + entity`
- query text that simply restates the full target answer

The key principle is:

- the query may be informative
- but it should still behave like a retrieval request, not an answer-derived key

### For Task B: Reduced-Slot Retrieval

Recommended constraints:

- include at most one or two core slots
- do not include all discriminative fields
- avoid directly copying the target goal
- keep the query natural and short

The key principle is:

- the query should remain valid
- but should require more semantic discrimination than Task A

## 7. LoRA Training Data

LoRA training examples should be constructed only from Task A.

Each training example should contain:

- one query
- one positive target episode
- one or more hard negative episodes

Hard negatives should preferably satisfy:

- same scenario
- same or similar tool trace family
- only one or two discriminative field differences

This is the most important part of the LoRA setup.

The goal is not to teach the model the scenario label.
The goal is to teach the model to separate extremely similar episodes.

## 8. Recommended Metrics

The following metrics should be reported for both base and LoRA models.

### On Task A

- `Recall@1`
- `Recall@5`
- `MRR`
- same-scenario `Recall@1`
- hard-negative subset `Recall@1`

### On Task B

- `Recall@1`
- `Recall@5`
- `MRR`

### Optional

- cluster-level accuracy
- average rank of the correct episode within same-scenario candidates

The most important metric for the LoRA story is:

- improvement on hard same-scenario retrieval

## 9. Recommended Baselines

At minimum, the experiments should compare:

1. lexical baseline or simple token-overlap baseline
2. base embedding retriever
3. LoRA-tuned embedding retriever

This comparison is important because:

- if lexical retrieval is already too strong, the benchmark may still be too easy
- if LoRA only beats lexical by a small margin, the conclusion should be cautious

## 10. Expected Outcomes

The ideal outcome is not necessarily a huge gain on every metric.

A good outcome would be:

- little or moderate gain on Task A overall
- clearer gain on Task A hard-negative subsets
- some transfer, but not necessarily large transfer, to Task B

This result would support the claim that:

- LoRA mainly improves fine-grained episode discrimination
- not just generic retrieval quality

Even if Task B gains are small, that is still acceptable.
It may simply show that the model was tuned for precise retrieval rather than weak-cue recall.

## 11. How to Write This in the Paper

The paper should describe the setup like this:

1. The primary task is precise agent-guided episode retrieval under partially known cues.
2. A secondary reduced-slot benchmark is introduced to test generalization to weaker queries.
3. LoRA is trained only on the primary task to improve hard-negative discrimination.
4. Transfer to the secondary task is evaluated without claiming that the model was directly optimized for it.

This framing is narrow, clean, and realistic for a course project.

## 12. Final Recommendation

For the current project, the simplest defensible experiment is:

1. keep Precise Retrieval as the main benchmark
2. add Reduced-Slot Retrieval as an auxiliary benchmark
3. remove backfill queries from the main headline evaluation
4. train LoRA only for hard-negative discrimination on Precise Retrieval
5. report whether that LoRA transfers to Reduced-Slot Retrieval

This design is strong enough to support a meaningful project while still remaining feasible for one student.
