---
title: Lumilio Agent
description: "Organize media in natural language: build albums from selected photos, tag in batches, confirm before applying, receipts as proof."
---

# Lumilio Agent

Lumilio Agent is the conversational organizing assistant: describe what you want in natural language, and it works within **your current account and authorized repositories**. Every operation that changes organization requires **precise confirmation**, and the result is authoritative only when the server returns a **receipt**.

## Typical use

**Build an album from selected photos**:

1. Select a batch of photos in the library (the selection scope is shown next to your message);
2. Ask: “Put these photos into an album called Beach Holiday”;
3. Check the impact scope the Agent shows (target album, media count);
4. Click **confirm** — this only submits; the interface shows `submitting → committed / rejected / failed / cancelled`;
5. Only after the server returns the **effect receipt** is the operation applied.

Other real tasks: **batch tagging** (“add the tag Trip 2024 to these”), **review results** (“list duplicates among these”), **organize** (“group the selected media by theme”). Cancelling, stopping, or closing the conversation **never commits** interrupted changes.

## What is shown next to your message

Each user message shows the **mode, selection/viewing scope, and mentions**. **Dropped mentions are not silently ignored**: mentions that cannot be resolved or exceed permissions are marked explicitly so you can correct them.

A result set is a **working snapshot** of the current query and permissions; new imports, deletions, or permission changes can make it differ from a later browse page. To keep a result, **pin it to the Board** — conversation memory is temporary; **pins, the Board, media changes, and receipts are the persistent facts**.

## States: not configured, disabled, unreachable

- **Not configured**: no LLM provider or model chosen; the entry says so explicitly;
- **Disabled**: the administrator turned off “Enable Lumilio Agent”;
- **Unreachable**: configured but connection fails, the secret is invalid, or validation did not pass.

All three states are shown honestly. **The base media library does not depend on the Agent** — importing, browsing, and organizing keep working in every state.

## Configuration and privacy

Administrators configure it under **Settings → AI** (see [AI settings](./settings)):

- The provider is **chosen explicitly**; no default fallback. DeepSeek/Ollama need a **Base URL**; remote providers need an **API key**;
- **Verify connection** validates the unsaved draft only; failure saves nothing. **Switching providers requires providing the secret again**;
- “Enable the Agent” and “Verify the provider” are two different actions.

**The data boundary depends on the provider**: with a local provider (e.g. Ollama) your questions and referenced metadata stay on your device; a remote provider receives your questions, referenced context, and tool results — whether they leave your device depends on its data-retention rules. Review privacy policy and fees before enabling.
