---
title: Lumilio Agent
description: "Learn about Lumilio Agent's controlled media-management loop: explicit media scope, observable tool execution, precise confirmation, and authoritative receipts."
---

# Lumilio Agent

Lumilio Agent is the conversational organizing assistant inside Lumilio Photos: describe what you want in natural language, and the Agent calls organizing tools **within your current account and authorized repositories**. It is not a chat toy; it is a controlled media-management loop:

```text
Define the media scope → Ask / choose a mode → Observable tool execution
→ Confirm the exact impact → Authoritative effect receipt → Review the result
```

The Agent is independent of [Lumen Intelligence](./lumen-intelligence): the Agent needs an LLM provider (see [Configuration, providers, and privacy](#configuration-providers-and-privacy) below), while Lumen Intelligence provides Image Semantic Analysis, Person Recognition, OCR Text Recognition, and BioCLIP Species Recognition. **The base media library depends on neither**: when the Agent is not configured, importing, browsing, albums, and backups keep working.

## Where to open Lumilio Agent

- The Agent page in the sidebar;
- The global floating entry;
- The ask entry inside the media viewer.

All entries use the same account permissions and repository scope. Without a login, repository access, or a usable LLM configuration, Lumilio Agent never bypasses these limits to read media.

## Step 1: Define the media scope

The Agent usually cannot “just know” which media you mean. Say the scope explicitly, or start from selected media:

- State the scope directly: “photos in the **2024 trip album**”, “videos in the **Seaside** collection”;
- Select a batch of photos in the library and then ask; the session remembers that selection;
- Mention (reference) people, albums, collections, cameras, lenses, events, or pinned items.

**Each user message shows the mode, the selection/viewing scope, and the mentions beside it**, so you can confirm that the Agent is looking at the media you intend. **Dropped mentions are not silently ignored**: if a mention cannot be resolved or is outside your permissions, it is marked explicitly next to the message so you can correct it before continuing.

A result set is a working snapshot of the current query and permissions, not a permanent filter. New imports, deletions, trash restores, or permission changes can make it differ from a browse page opened later.

## Step 2: Choose a mode and watch the tools run

Pick a mode that matches the task, for example **Organize** (build albums, tag in batches, batch operations) or **Analyze** (statistics, review). Tool calls are **observable**: the interface shows which tool is running and how many media items it has processed, and you can stop generation at any time.

Cancelling, stopping generation, or closing the conversation never commits interrupted changes.

## Step 3: Confirm the exact impact

Operations that change organization (create albums, add tags, batch-like, and so on) stop at a **needs confirmation** state. Before confirming you can preview the impact scope; at least check:

- the target repository and current account;
- whether the result-set count matches what you asked for;
- whether the operation adds or removes, and which media it affects;
- whether existing album membership or trash state is affected.

**Clicking confirm only submits the request**; it does not mean the operation has taken effect. After confirmation the interface shows `submitting → committed / rejected / failed / cancelled`; only when the server returns an **effect receipt** can you treat the operation as applied.

## Step 4: The effect receipt is authoritative

- The receipt is issued by the server after the transaction commits; it is the **authoritative proof** that the operation succeeded — not the natural-language text in a model reply;
- Receipts are kept for 30 days after each run and can be **reconciled** per user, conversation, and effect: who did what, to which media, when;
- For concurrent confirmations, only effects that are still pending and unbound are executed, so the same media is not operated on twice.

## Working memory vs. persistent facts

Working memory inside a conversation is **temporary**: the Agent does not remember earlier discussions after you close the conversation or start a new one. What persists:

- what you pinned to the Board;
- pinned references and result sets;
- actual media changes (album membership, tags, likes);
- server-issued effect receipts.

To keep a result, pin it to the Board or run the operation — do not rely on the conversation transcript.

## States: not configured, disabled, and unreachable

The Agent has three distinct availability states:

- **Not configured**: no LLM provider or model has been chosen. The entry says so explicitly; it never pretends to be available;
- **Disabled**: the administrator turned off “Enable Lumilio Agent”. The entry is visible but unusable;
- **Unreachable**: a provider is configured but connection fails, the secret is invalid, or server-side validation did not pass.

These states are shown honestly. The base media library does not depend on Lumilio Agent, and importing, browsing, and organizing keep working in every state.

## Configuration, providers, and privacy

Administrators configure the Agent under **Settings → AI** (see the [AI settings](./settings.md)):

- **The LLM provider must be chosen explicitly**: there is no silent fallback to a default service; unknown or empty values are treated as not configured;
- **Verify connection** validates the current unsaved draft; a failed verification saves nothing;
- **Switching providers requires providing the secret again**: a saved API key belongs to the previous provider only;
- DeepSeek / Ollama require an explicit **Base URL**; remote providers require an **API key**.

**The data boundary depends on the provider and deployment**:

- **Local provider** (for example Ollama): your questions and referenced media metadata stay on your device;
- **Remote provider**: your questions, referenced context, and tool results are sent to that service; whether they leave your machine depends on its data-retention rules.

Before enabling a remote provider, review its privacy policy, fees, and data-retention rules. Lumilio keeps account, network, and repository-sensitive information in the model context to the minimum required for the task.

For references, result-set snapshots, the Board, and confirmation boundaries, see [Lumilio Agent in depth](../help/agent-details).
