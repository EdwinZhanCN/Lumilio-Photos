# Decision: Converge LLM providers on native Eino adapters

Status: implemented

## Problem

Lumilio pinned Eino `v0.9.6`, exposed only four LLM provider IDs, and built
DeepSeek through the OpenAI-compatible adapter. Provider membership and
configuration requirements were duplicated across Server validation, DTOs,
service logic, and Web code. Some upstream SDKs can also discover credentials
from ambient process state, which conflicts with Lumilio's explicit encrypted
settings ownership.

The provider expansion also had to preserve the existing classic-message ADK
contract: streaming and tool execution, cancellation, summarization,
confirmation checkpoints, resume, session persistence, audit behavior, and
already-constructed models used by in-flight runs.

## Decision

Pin Eino core and ADK to stable `v0.9.14` and retain the
`schema.Message`/`model.ToolCallingChatModel` path. Lumilio supports exactly
Ark, OpenAI, DeepSeek, Ollama, Claude, Gemini, Qwen, and OpenRouter through
their reviewed official Eino-ext classic adapters. DeepSeek uses the native
DeepSeek adapter. Agentic providers, hosted variants, arbitrary
OpenAI-compatible endpoints, and Qianfan are outside this boundary.

The Server settings package owns the provider descriptor registry, including
stable ID and API-key/base-URL requirements. Validation, model construction,
the Settings DTO, generated OpenAPI types, and the Web consume this contract;
they do not maintain independent provider allowlists. The existing single
encrypted API-key aggregate remains authoritative, and changing provider
clears the prior stored key unless the same update supplies its replacement.

Complete Lumilio settings are validated before an adapter or underlying SDK is
constructed. Runtime code does not use environment variables, credential
files, provider singletons, or default credential chains to repair incomplete
settings. Adapter errors are normalized at the provider boundary so ordinary
logs cannot contain raw response bodies, authorization headers, credentials,
or full prompts.

Repository support is proven by deterministic, provider-shaped tests for all
eight adapters, including generation, streaming, and a streamed tool-call and
tool-result round trip. DeepSeek additionally proves native component identity
and reasoning content. Credentialed public-provider smoke tests are not kept
as a repository suite or release gate: external secrets, service availability,
model entitlements, and network access are deployment concerns rather than a
deterministic provider-contract invariant.

Eino `v0.9.6` confirmation checkpoints remain resumable on `v0.9.14` through
the SQLite store. Checkpoint writes always supply their required update
timestamp. Provider changes affect subsequently constructed models while
in-flight Agent runs retain the model instance with which they started.

The detailed current boundary is documented in
[`BACKEND.md`](../../site/docs/internal/BACKEND.md#ml-lumen-and-llm).

## Alternatives considered

**Continue routing DeepSeek through OpenAI compatibility** — rejected because
compatibility is not native adapter support and obscures provider identity and
DeepSeek-specific response behavior.

**Keep provider allowlists in every layer** — rejected because the Server,
OpenAPI, generated Web types, UI choices, and validation requirements can
drift. One Server registry gives every consumer the same safe descriptor.

**Let SDK defaults or environment variables fill missing settings** — rejected
because ambient state bypasses Lumilio's encrypted settings ownership and can
change behavior between processes or deployments.

**Adopt Eino `v0.10` alpha or migrate to AgenticModel while adding providers**
— rejected because it would combine a persisted-message/API migration with a
provider-boundary upgrade and use a prerelease contract.

**Add Qianfan and hosted provider variants under the nearest existing ID** —
rejected because Qianfan relies on process-global and multi-form credential
configuration, while hosted variants have distinct authentication and runtime
contracts. They require explicit future schemas and provider identities.

**Require credentialed live smoke tests for every provider** — rejected
because they depend on secrets, paid model access, public network state, and
provider uptime, none of which is a reproducible repository gate. The
deterministic adapter fixtures cover the product contract without retaining a
credential-bearing test harness.
