# Design

This document captures product and interface design guidance for Lumilio Photos. It is based on the current codebase and should stay practical.

## Product Shape

Lumilio Photos is a local-first media management app. The product should feel like a workspace for real photo libraries, not a marketing surface.

Primary jobs:

- browse and inspect media quickly
- preserve original files and repository structure
- upload, ingest, scan, and process media predictably
- organize albums, people, places, stacks, and duplicates
- expose ML/AI assistance without making it mandatory
- make system status, settings, and failures understandable

## Interface Character

The UI should be calm, dense, and operational. Prefer clear hierarchy and repeatable workflows over oversized hero sections or decorative screens.

Use:

- compact navigation
- predictable route structure
- scan-friendly panels
- stable dimensions for grids, thumbnails, toolbars, and media viewers
- responsive layouts that keep controls reachable
- icons for common actions
- text only where it clarifies a command or domain concept

Avoid:

- marketing-page composition inside the app
- decorative gradients, orbs, and low-information cards
- nested cards
- layout shifts caused by dynamic labels or loading states
- duplicated controls that imply different ownership of the same state

## Domain UX Principles

Media browsing:

- The asset should be the center of attention.
- Thumbnails and grids need stable sizing.
- Selection, filtering, and navigation state should be easy to recover.
- Full-screen inspection should keep metadata/actions available without crowding the image.

Upload and ingest:

- Make progress and failure states explicit.
- Do not imply original files are altered unless they are.
- Retry paths should be visible and deterministic.

Collections:

- Albums, people, places, and duplicate utilities should share interaction patterns where possible.
- Avoid separate one-off mental models for similar media lists.

Settings and monitor views:

- Optimize for diagnostics and repeated use.
- Dense tables/forms are acceptable when the user is doing system work.
- Errors should identify the subsystem involved: API, storage, queue, ML, auth, or cloud sync.

AI and ML:

- Present AI as assistive.
- Empty, disabled, or unavailable ML states should still leave the base media workflow usable.
- Avoid UI that makes remote models feel required for local library management.

## Component Guidance

- Prefer existing feature components before creating new shared abstractions.
- Put domain behavior in feature hooks/services, not generic UI components.
- Keep pages thin; route components should compose hooks and feature components.
- Use `lucide-react` for icons.
- Keep user-facing copy i18n-ready through the existing i18n layer.
- Use TanStack Query lifecycle states instead of hand-rolled loading/cache state.

## Visual Quality Checks

Before finishing UI work, check:

- no text overlap at desktop and mobile widths
- controls keep stable size across loading/empty/error states
- interactive targets are visible and reachable
- server-state loading and error states are represented
- generated media or WASM-backed output is not blank
- route-level layout remains usable with sidebar and navbar present

## Design Debt

Track durable design debt in `docs/agent/exec-plans/tech-debt-tracker.md`, not in scattered TODOs that do not identify a path or user impact.
