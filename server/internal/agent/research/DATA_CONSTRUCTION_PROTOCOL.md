# Data Construction Protocol

This note defines the revised data construction protocol for the episodic memory retrieval benchmark.

The purpose of this protocol is to solve the instability of the old split-first query coverage while keeping the evaluation methodologically clean.

## Core Principle

The benchmark should be constructed in two stages:

1. split the **episodes**
2. generate the **queries** independently for each split

After query generation is complete, the test split must be frozen and must not be modified during model development.

This design improves test coverage without using test results to shape the test set after evaluation has already begun.

## Revised Protocol

### Step 1. Generate the full episode set

First generate the complete set of candidate episodes.

At this stage, the dataset should contain:

- `episode_id`
- `cluster_id`
- `scenario`
- `intent`
- episode content fields
- metadata
- tool traces

The output of this step is an **episode-only corpus**.

It is acceptable if query generation has not yet happened.

### Step 2. Split episodes by cluster

Next, split the episodes into:

- `train`
- `val`
- `test`

The split unit must be the `cluster`, not the individual episode.

This is important because episodes inside the same cluster are intentionally minimal-difference variants. If they were split independently, near-duplicate leakage across splits would occur.

After this step, each split should contain only episodes.

### Step 3. Generate queries independently for each split

After the episode splits are fixed, generate queries separately for:

- train episodes
- validation episodes
- test episodes

The query generation process must treat each split independently.

This means:

- train queries are generated only from train episodes
- validation queries are generated only from validation episodes
- test queries are generated only from test episodes

The goal is to ensure that each split has enough query coverage without relying on a query set that was generated globally before splitting.

### Step 4. Freeze the test set

Once test queries have been generated, the test split must be frozen.

After freezing:

- test queries should not be regenerated
- test episodes should not be rebalanced
- no test query should be added or removed based on observed model performance

This is the most important methodological constraint in the protocol.

### Step 5. Restrict development to train and validation only

During model development:

- training uses only the training split
- model selection and prompt tuning use only the validation split
- test is reserved for final evaluation only

This applies to:

- retriever changes
- LoRA tuning
- negative sampling strategy changes
- prompt changes for generation or reranking

### Step 6. Report test only after development is complete

The test split should be evaluated only after:

- the training pipeline is fixed
- validation-based model selection is complete
- benchmark settings are finalized

The test result should be reported as the final held-out evaluation.

## Why This Protocol Is Better

The old pipeline generated a global query set first and split it afterward by target episode.

That approach had one major weakness:

- the final number of test queries was only an indirect consequence of the upstream generation process

As a result:

- some test splits ended up with too few queries
- query coverage across test clusters was unstable
- later synthetic backfill was needed to patch missing coverage

The revised protocol is better because:

1. the split boundary is defined using episodes and clusters first
2. query generation is aligned with the actual split structure
3. test coverage can be improved in a controlled way
4. the test set is still frozen before model tuning

## Why This Does Not Violate Evaluation Discipline

This protocol does **not** mean that the test set is being modified in response to model performance.

The critical distinction is:

- acceptable: define the split first, then generate the benchmark queries once, then freeze the test set
- unacceptable: inspect test performance, then change test queries to make the benchmark easier or better shaped for the model

Therefore the methodological rule is not:

- "test queries must be created before splitting"

The correct rule is:

- "test queries must be finalized before model development and must not be changed based on test outcomes"

## Recommended Interpretation in the Paper

This protocol can be described in the paper as follows:

> We first generated the full episode corpus and split it into train, validation, and test partitions at the cluster level. We then generated retrieval queries independently for each split. After the split-specific query sets were created, the test split was frozen and used only for final evaluation, while model development and tuning were performed exclusively on the training and validation splits.

This wording is clear and methodologically defensible.

## Practical Consequences for This Project

Under this protocol:

- test query count becomes directly controllable through split-specific generation
- synthetic template backfill is no longer necessary as the main fix for missing test coverage
- train, validation, and test can each have their own query distributions while remaining cleanly separated

This also makes later benchmark variants easier to support, such as:

- precise retrieval queries
- reduced-slot retrieval queries
- scenario-balanced query generation

## Final Rule

The final rule is simple:

- **split episodes first**
- **generate split-specific queries second**
- **freeze test before model tuning**

This should be the default benchmark construction protocol going forward.
