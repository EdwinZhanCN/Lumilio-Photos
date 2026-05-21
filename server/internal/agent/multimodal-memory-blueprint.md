# Sparse Multimodal Memory Blueprint

This note captures the current direction for Lumilio's VLM caption work. It is a conceptual reference, not an implementation plan.

## Current Agent Context

Lumilio's agent runtime already has the right extension point for multimodal memory:

- `core.AgentService` builds an Eino ADK `ChatModelAgent`, runs it with checkpoint support, and streams agent events.
- `ToolRegistry` owns server-side tool registration. Tools are selected by name and constructed with request-scoped dependencies.
- `ToolDependencies` gives tools database access, a side channel, and a `ReferenceManager`.
- `ReferenceManager` lets tools pass structured state through stable `ref.*` IDs instead of asking the LLM to reproduce large objects.
- The current HTTP/SSE handler is only an adapter around the agent runtime. Future memory access should be an agent tool, not a separate public caption API.

The important consequence: VLM captions should not be designed primarily as user-visible media metadata. They should be designed as internal memory material for tools and agents.

## Mental Model

Lumilio should treat media understanding as two layers.

### Sparse Multimodal Memory

Sparse memory is cheap, cached, text-first context derived from media:

- CLIP semantic embeddings
- OCR text
- BioCLIP labels
- face metadata
- timestamps, location, filenames, albums
- VLM memory captions

This is the high-throughput layer an LLM agent can reason over during normal retrieval and planning.

### Dense Visual Working Memory

Dense working memory is direct image inspection by a stronger multimodal model or by the agent's own multimodal capability. This is slower and more expensive, so it should be used only after retrieval has narrowed the candidate set to a few assets.

The original image remains the source of truth. Sparse memory is a retrieval and reasoning scaffold, not visual ground truth.

## VLM Task Definition

The VLM work should be split into two runtime tasks:

1. `vlm_embeds`
   - Input: preprocessed image tensor plus one fixed English memory prompt.
   - Output: binary `merged_inputs_embeds`.
   - Purpose: cache the expensive image+prompt preparation step.

2. `vlm_decode`
   - Input: cached `merged_inputs_embeds`.
   - Output: `TextGenerationV1`.
   - Purpose: materialize a memory caption only when a retrieval or agent flow actually needs it.

The prompt is baked into the embeds. Language is not a separate axis. For this memory layer, English-only captions are acceptable because the consumer is the LLM agent, not the end user.

## Fixed Memory Prompt

Use one backend-owned English prompt. Do not let the frontend supply this prompt for the memory pipeline.

The prompt should produce agent-useful observations rather than a polished human caption. It should favor:

- concrete visible facts
- objects, people, actions, relationships, and scene layout
- visible text if present
- uncertainty when details are ambiguous
- avoiding identity or intent guesses unless supported by metadata
- compact but information-rich wording

Prompt versioning is intentionally out of scope while the product is in active development. If the fixed prompt changes, existing VLM embed artifacts and captions can be cleared and regenerated.

## Artifact Identity

Because there is one fixed memory prompt, the VLM embeds artifact identity can stay simple:

```text
asset_id + model_id + preprocess_version
```

The artifact filename can follow:

```text
{asset_id}_{model_id}_{preprocess_version}_vlm_embeds.bin
```

The repository storage location should be system-managed, for example:

```text
.lumilio/assets/vlm_embeds/
```

That matches the existing repository model where `.lumilio/` is protected and generated assets such as thumbnails, transcodes, and face crops live under `.lumilio/assets/`.

The database should store metadata for the artifact, while the binary stays in repository storage.

## Caption Materialization

The database `captions` table is still a good place for the decoded text. In this model, a caption means:

> a sparse memory text generated from a cached VLM embeds artifact

It is not primarily a user-facing caption.

Decode should be on-demand:

1. Semantic search retrieves a larger candidate set.
2. A future agent memory tool selects a small top subset, for example top 10.
3. For each selected asset:
   - if a caption exists, return it;
   - if missing, load the VLM embeds artifact, run `vlm_decode`, save the caption, and return it.
4. The agent reasons over caption text plus OCR, tags, dates, location, thumbnails, and asset IDs.
5. If exact visual truth matters, the agent escalates to dense inspection for one or two assets.

This keeps normal agent reasoning fast while preserving a path to deeper visual understanding.

## Pipeline Role

The ML pipeline should focus on preparing the sparse memory substrate:

```text
photo thumbnail -> caption preprocessing -> vlm_embeds -> .bin artifact
```

The pipeline should not eagerly decode captions for every image unless a future product requirement proves that worthwhile. Eager decode creates text for assets that may never appear in agent reasoning.

Current caption indexing can be reinterpreted as "VLM memory readiness":

- ready means the asset has a valid `vlm_embeds` artifact for the current model and preprocess version
- decoded caption means memory has been materialized for prior retrieval/agent use

## Agent Tool Shape

The future agent-facing unit should be a tool, not a public API.

Conceptually, that tool should:

- accept a query or a reference to prior retrieval results
- run semantic retrieval or consume the referenced candidate set
- materialize memory captions for a bounded top subset
- return compact text to the LLM
- optionally store richer candidate data in `ReferenceManager`
- use side-channel events only for structured progress or frontend rendering, not as the memory contract itself

The LLM should receive concise memory records, for example:

```text
asset_id: ...
score: ...
memory_caption: ...
ocr: ...
tags: ...
date/location: ...
```

The tool should also expose when dense inspection is recommended, especially when the user asks about exact counts, fine details, safety-critical decisions, or visual distinctions between similar images.

## Non-Goals

- No standalone public caption generation API.
- No user-custom prompt support for the sparse memory pipeline.
- No multilingual caption strategy for this layer.
- No prompt profile system until there is a concrete product need.
- No assumption that sparse captions are authoritative visual truth.

## Guiding Principle

Semantic search narrows the world. VLM decode materializes sparse memory for the few assets worth reasoning about. Dense multimodal inspection is reserved for the final one or two assets where visual truth matters.
