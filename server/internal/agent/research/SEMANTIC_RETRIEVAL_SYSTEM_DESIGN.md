# Semantic Retrieval System Design

This note documents the current retrieval design for the agent episodic memory benchmark and the reasoning behind the latest split changes.

## Design Goal

The primary goal is to evaluate semantic episode retrieval rather than oracle-assisted lookup.

The retrieval system should answer the following question:

> Given a natural-language recall query, can the memory engine retrieve the correct prior episode rather than a merely similar task family?

This is an instance-level retrieval problem, not only a task-category classification problem.

## Three Retrieval Layers

The memory system is organized into three layers with different responsibilities.

### 1. Dense Retrieval Text

This is the text sent to the embedding model. It is intended to act as the agent's "hippocampal cue" for dense semantic recall.

It should contain only the information that is useful for semantic retrieval:

- `What`: `scenario`, `intent`, `summary`
- `Goal`: the explicit task objective
- `Task content`: sparse task-defining entities
- `Procedure`: tool execution sequence

It should not contain fields that mainly function as shortcuts, bookkeeping artifacts, or result dumps.

### 2. Episode Metadata

This layer stores contextual information about the episode itself, but does not necessarily enter the dense embedding text.

Examples:

- `started_at`
- `ended_at`
- `workspace`
- `route`
- other structured payload fields

This metadata is useful for auditability, provenance, later reranking, and future runtime filtering, but it should be separated from the dense semantic representation.

### 3. Runtime Constraints

This layer contains constraints that are available only at runtime from the actual agent context.

Examples:

- current workspace
- current route
- user/session scope
- time constraints parsed from the live request

These constraints are valid only if they come from real runtime state. They should not be injected from gold evaluation annotations.

## Why Filters Were Removed from the Benchmark Default

The earlier benchmark path allowed retrieval filters derived directly from dataset query annotations such as `entity` and `status`.

That design inflated retrieval scores because the benchmark was no longer testing pure semantic retrieval. Instead, it combined dense retrieval with oracle-provided structured constraints.

The benchmark now defaults to `use_filters = False` so that the reported scores reflect semantic retrieval behavior more faithfully.

## Dense Retrieval Text Template

The dense retrieval text is now structured into four sections:

```text
what:
scenario=<scenario>
intent=<intent>
summary=<summary>

goal:
<goal>

task_content:
<type1>=<name1>
<type2>=<name2>

procedure:
<tool1> -> <tool2> -> <tool3>
```

This representation keeps the retrieval cue compact and semantically aligned with instance recall.

## What Counts as Task Content

Entities in this project are not intended to represent the full result set of an episode.

Instead, they are sparse task-defining items such as:

- location
- album
- camera model
- failure mode
- selection theme
- time window

These are part of the content of the task, not the full output of the task.

## Context vs. Content

The benchmark now explicitly distinguishes between:

- **episode context**
  - `When`: `started_at`, `ended_at`
  - `Where`: `workspace`, `route`

- **task content**
  - `What`: `scenario`, `intent`, `summary`
  - content entities such as location, album, camera, failure mode, and time window

- **agent-specific retrieval cues**
  - `Goal`
  - `Tool Trace`

This separation is important because fields such as `entities.location` or `step.input.time_range` describe the media task being handled, not the operational context in which the agent episode occurred.

## Benchmark Split Design

The benchmark retains the principle that:

- one query should map to one target episode

This preserves instance-level evaluation.

However, the earlier `baseline-v6` split left only one test cluster per scenario, which collapsed the test benchmark to only seven queries. That was too small for a stable evaluation.

The split design has now been updated so that:

- each scenario keeps at least `3` test clusters by default
- each test episode is guaranteed at least one query

This preserves the original hard-negative design while greatly increasing benchmark size.

## Why Cluster-Level Splitting Still Matters

Episodes are grouped by `scenario + cluster_id`, where each cluster contains minimal-difference variants of the same underlying task pattern.

Keeping clusters intact during splitting ensures that:

- hard negatives remain meaningful
- leakage between train and test is reduced
- evaluation still measures instance discrimination, not only task-family recognition

## Expected Outcome

This design should produce a benchmark that is both:

- cleaner from a retrieval-methodology perspective
- harder to game through annotation leakage

It also creates a better foundation for any future LoRA or contrastive-learning experiments, because improvements can be measured against a stronger semantic retrieval protocol rather than a shortcut-heavy baseline.
