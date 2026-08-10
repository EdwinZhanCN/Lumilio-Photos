---
title: Lumilio Agent in depth
description: How Lumilio Agent references work, why result sets are snapshots, and where confirmation and receipt boundaries live.
---

# Lumilio Agent in depth

For daily use see [Lumilio Agent](../features/agent). This page explains why the Agent returns result sets first and which operations must wait for confirmation.

## References and result sets

Lumilio Agent creates references (refs) from the current user, conversation thread, and authorized repositories, and the interface loads the media those references point to. A reference is not a new repository and never expands the access scope the user already has.

A result set is best treated as a working snapshot of one query: after new imports, deletions, restores, permission changes, or original filter changes, the count can differ. Items pinned to the Board can keep displaying or refresh, but they remain bound to the original references and account permissions.

## Checks before confirmation

Tool calls that change organization — creating albums, adding tags, batch-like, and so on — must enter a **needs confirmation** state first. When the confirmation window appears, check:

- the result-set count and repository scope;
- whether the operation adds or removes;
- whether existing album membership would change;
- whether the current account should perform the operation at all.

Cancelling, stopping generation, or closing the page never commits interrupted changes. Do not treat natural-language text in a model reply as proof of success; the final authority is the page result and the actual state in the repository — see the [effect receipt](../features/agent) section on the Agent page.

## LLM configuration and the data boundary

Lumilio Agent uses the LLM provider, model, Base URL, and API key configured under **Settings → AI**. Lumen Intelligence nodes and the Agent's LLM provider are two different data paths: the former handles media-derived data, the latter receives your questions, referenced context, and tool results.

Before enabling a remote LLM, review the privacy policy, fees, data retention, and where the API key is stored. Unnecessary account, network, or repository-sensitive information should not appear in the model context.
