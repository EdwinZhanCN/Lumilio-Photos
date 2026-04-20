# Episodic Memory Field Mapping for Agent Memory

This note defines the current conceptual mapping between classical episodic memory and the structured fields used in this project.

## Motivation

Classical episodic memory is often described through the dimensions of `What`, `Where`, and `When`. That framing is useful, but it must be adapted carefully for an automated agent system.

For a human, `Where` and `When` usually refer to the physical and subjective context of an experience. For an agent, however, some fields that look like location or time may actually describe the content of the task rather than the context of the episode itself. For example, a photo `location` or `time_window` may refer to the media being processed, not to when or where the agent episode occurred.

Because of that distinction, this project adopts a stricter interpretation of episodic memory for agents.

## Proposed Mapping

### When

`When` should refer only to the temporal position of the agent episode itself.

In this project, the correct realization of `When` is:

- `started_at`
- `ended_at`

Optional step-level timestamps may support finer-grained analysis, but the core episode-level memory should anchor `When` to the actual execution time of the episode.

Fields such as `time_window` or `step.input.time_range` should not be treated as episodic `When`. They are semantic properties of the task content, not the temporal context of the agent's own experience.

### Where

`Where` should refer to the operational and execution context in which the agent episode took place.

In this project, the most appropriate realization of `Where` is:

- `workspace`
- `route`

Here, `workspace` captures the operational context of the episode, while `route` captures the execution path or subsystem context through which the episode was produced.

Fields such as `entities.location` should not be treated as episodic `Where`. They describe the content of the task, such as the place associated with the photos being processed, rather than the context in which the agent itself operated.

### What

`What` should capture what happened in the episode at the task level.

In this project, the strongest realization of `What` is:

- `scenario`
- `intent`
- `summary`

These fields together describe the type of task, the action pattern, and the episode-level abstraction of what occurred.

## Agent-Specific Augmentations

For agent memory, the classical `What/Where/When` formulation is useful but incomplete. Two additional dimensions are especially important:

### Goal

`Goal` captures why the agent performed the episode.

Unlike `What`, which describes what happened, `Goal` describes the intended outcome of the episode. This is a core retrieval cue for agent memory because many future retrieval requests are goal-driven.

### Tool Trace

`Tool Trace` captures how the episode unfolded procedurally.

This includes the ordered sequence of tools or operations used during execution. For agent systems, procedural structure is often as important as semantic similarity. Two episodes may involve similar media content but differ substantially in how the agent solved the task.

## Resulting Agent Memory Structure

Under this formulation, the episodic memory representation for an agent is:

- `What`: `scenario`, `intent`, `summary`
- `Where`: `workspace`, `route`
- `When`: `started_at`, `ended_at`
- `Goal`: explicit task objective
- `Tool Trace`: ordered execution pattern

This yields a `3 + 2` formulation: the classical episodic memory core (`What/Where/When`) plus two agent-specific dimensions (`Goal/Tool Trace`).

## Design Implication

This mapping also clarifies what should not automatically be treated as episodic context.

- `entities.location` is task content, not episodic `Where`
- `entities.time_window` is task content, not episodic `When`
- `step.input.time_range` is a retrieval or task constraint, not episodic `When`

These fields may still be valuable as semantic content features, but they should not be confused with the contextual dimensions of the episode itself.

## Summary

The core claim is simple:

- for agents, episodic `When` is the time of the episode itself
- episodic `Where` is the operational and execution context
- episodic `What` is the abstract task-level content of the episode
- `Goal` and `Tool Trace` should be modeled explicitly as agent-specific memory cues

This formulation is better aligned with the nature of automated systems than a direct reuse of human episodic memory categories without adaptation.
