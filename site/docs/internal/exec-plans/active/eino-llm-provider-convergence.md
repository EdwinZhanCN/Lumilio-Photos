# Eino ADK and LLM Provider Convergence

Status: active. Upstream support and target contracts were investigated and
frozen on 2026-08-17. No implementation phase has started.

Primary owners: `server/internal/llm`, `server/internal/settings`,
`server/internal/agent/core`, `server/internal/service/settings_service.go`,
`server/internal/api/dto/settings_dto.go`, `web/src/features/settings`,
`server/go.mod`, and `desktop/go.mod`.

Lumilio currently pins `github.com/cloudwego/eino v0.9.6` and exposes Ark,
OpenAI, DeepSeek, and Ollama. Only Ark, OpenAI, and Ollama use their Eino-ext
provider adapters. DeepSeek is routed through the OpenAI-compatible adapter,
and the architecture gate forbids the native DeepSeek dependency because an
older transitive SDK path could discover credentials from ambient process
state. Provider IDs and provider-specific validation are then duplicated in
the settings domain, DTO annotations, service logic, and Web code.

## Goal

Upgrade Lumilio's existing Eino ADK/classic-message agent path to the latest
stable Eino line and make the product provider surface match the official
Eino-ext adapters that can honor Lumilio's explicit settings and secret
ownership contracts.

The observable end state is:

- `github.com/cloudwego/eino` is pinned to `v0.9.14`, which also upgrades
  `github.com/cloudwego/eino/adk`; ADK is a package in the Eino module, not a
  separately versioned dependency;
- Ark, OpenAI, DeepSeek, Ollama, Claude, Gemini, Qwen, and OpenRouter are
  selectable product providers and each is constructed through its official
  Eino-ext adapter;
- selecting DeepSeek constructs a native DeepSeek model, never an OpenAI model
  with a substituted endpoint;
- provider identity, credential requirements, endpoint requirements, and
  availability come from one Server-owned registry and the Web renders that
  contract instead of maintaining another provider allowlist;
- an empty or invalid Lumilio setting fails before any provider SDK can consult
  environment variables, credential files, cloud profiles, or other ambient
  state;
- the existing encrypted single-provider credential, explicit validation,
  streaming/tool execution, cancellation, summarization, checkpoint/resume,
  and audit behavior remain intact; and
- generated OpenAPI/TypeScript/docs and Desktop third-party notices describe
  the upgraded dependency graph without manual edits.

This plan upgrades the dependency and provider boundary. It is not a migration
of Lumilio conversations to Eino `AgenticMessage` or Eino-ext `AgenticModel`.

## Non-goals

- Do not adopt a `v0.10.0-alpha.*` Eino release. The latest stable release on
  the freeze date is `v0.9.14`; prerelease APIs do not define this plan.
- Do not migrate `adk.ChatModelAgent`, `schema.Message`, the conversation
  store, middleware, audit format, or public Agent stream to the AgenticModel
  path. That would be a separate persisted-message and API migration.
- Do not add Eino-ext agentic provider packages. Lumilio's current ADK path
  consumes classic `model.ToolCallingChatModel`; unused parallel adapters do
  not belong in the module graph.
- Do not expose Azure OpenAI, Claude on Bedrock, Claude on Vertex AI, Gemini on
  Vertex AI, Ark AK/SK authentication, or another hosted variant under an
  existing provider ID. Those variants need distinct credential and runtime
  contracts before they can be product options.
- Do not add Qianfan product support in this plan. Its official adapter
  configures credentials through a process-global SDK singleton and supports
  AK/SK or bearer-token forms, including environment-backed configuration.
  That cannot preserve Lumilio's per-setting encrypted credential ownership or
  isolate already-running Agent runs when an administrator changes provider.
- Do not add a generic arbitrary OpenAI-compatible provider, automatic model
  discovery, provider model catalogs, per-provider generation tuning, fallback
  routing, or multi-provider failover.
- Do not change Lumen inference, embeddings, image analysis, or other Eino-ext
  component families.
- Do not add a database migration. Every provider enabled here fits the
  existing selected-provider, model, base-URL, and single encrypted API-key
  aggregate.

## Upstream support snapshot (frozen 2026-08-17)

For this plan, "official Eino support" means a provider is named by the
[CloudWeGo Eino component guide](https://github.com/cloudwego/eino-ext/blob/main/skills/eino-component/SKILL.md),
has an implementation in the official
[Eino-ext model directory](https://github.com/cloudwego/eino-ext/tree/main/components/model),
declares the classic
`model.ToolCallingChatModel` contract, and has a stable non-prerelease module
tag. An OpenAI-compatible API alone is not counted as a native provider
adapter.

The [Eino `v0.9.14` release](https://github.com/cloudwego/eino/releases/tag/v0.9.14)
was published on 2026-08-13 and is the latest stable core/ADK release on the
freeze date. The `v0.9.6` to `v0.9.14` range includes ADK fixes for concurrent
message IDs, cancellation/checkpoint scopes, streaming tool-output reduction,
and summarization error context, as well as schema/compose fixes. Those are
behavioral reasons to exercise the Agent lifecycle, not permission to silently
change it.

The official classic ChatModel provider matrix is:

| Provider | Latest stable adapter | Lumilio disposition |
| --- | --- | --- |
| Ark | `components/model/ark v0.1.69` | Upgrade the existing native adapter. API-key auth only. |
| OpenAI | `components/model/openai v0.1.13` | Upgrade the existing native adapter. Direct OpenAI API only. |
| DeepSeek | `components/model/deepseek v0.1.7` | Add and replace the OpenAI-compatible construction path. |
| Ollama | `components/model/ollama v0.1.9` | Upgrade the existing native adapter. No API key. |
| Claude | `components/model/claude v0.1.25` | Add direct Anthropic API support. |
| Gemini | `components/model/gemini v0.1.33` | Add Gemini Developer API support with an explicitly constructed `genai.Client`. |
| Qwen | `components/model/qwen v0.1.9` | Add DashScope support. |
| OpenRouter | `components/model/openrouter v0.1.10` | Add direct OpenRouter support. |
| Qianfan | `components/model/qianfan v0.1.4` | Official upstream adapter, deferred from Lumilio for singleton/multi-secret ownership. |

`arkbot` is a specialized Ark bot component rather than one of the provider
implementations in the official ChatModel selection guide. The separately
named `agentic*` directories implement the AgenticModel path and are likewise
outside this plan.

Before changing `go.mod`, re-query the official release and tag endpoints. If
a newer stable version exists, update this dated snapshot deliberately, review
its release/source delta, and record the new freeze date in `Status:`. Never
turn the implementation phase into an unreviewed `@latest` upgrade.

## Fixed product provider contract

The enabled provider set and initial configuration requirements are:

| Provider ID | Adapter | API key | Base URL | Product boundary |
| --- | --- | --- | --- | --- |
| `ark` | `model/ark` | required | optional | Ark API-key authentication; no AK/SK mode |
| `openai` | `model/openai` | required | optional | Direct OpenAI Chat Completions path; no Azure mode |
| `deepseek` | `model/deepseek` | required | required | Native DeepSeek adapter; preserve the current explicit endpoint requirement |
| `ollama` | `model/ollama` | not used | required | Local/administrator-selected endpoint |
| `claude` | `model/claude` | required | optional | Direct Anthropic API; neither Bedrock nor Vertex |
| `gemini` | `model/gemini` | required | optional | Gemini Developer API; construct the Google client explicitly |
| `qwen` | `model/qwen` | required | required | DashScope compatible-mode endpoint |
| `openrouter` | `model/openrouter` | required | optional | Direct OpenRouter endpoint |

- Keep provider IDs lowercase and stable in SQLite and HTTP. Display names are
  localized Web copy; they are not stored values.
- Keep exactly one active encrypted API key. Changing provider clears the
  prior provider's stored secret unless a replacement is supplied in the same
  update, as it does today.
- An omitted optional base URL delegates only to the named adapter's documented
  service default. A supplied base URL is passed explicitly. A provider must
  never inherit another provider's endpoint.
- Provider-local SDK requirements that are not product choices remain internal
  constructor policy. In particular, direct Claude receives an explicit
  non-zero output-token limit; this does not create an undocumented settings
  field.
- Ollama is the only keyless provider in this plan. Every other enabled
  provider must fail Lumilio validation with an empty key before its adapter or
  underlying SDK is constructed.
- All eight adapters must satisfy the tool-calling and streaming path used by
  `adk.ChatModelAgent`. A provider with a text-only successful ping is not
  sufficient for product support.

## Fixed configuration and dependency contract

- `settings` owns a deterministic provider descriptor registry containing at
  least the stable ID, whether an API key is required, and whether a base URL
  is required. Validation, DTO projection, and the model factory consume that
  registry; there is no second Server allowlist.
- `llm` owns adapter construction. Provider packages do not leak into service,
  handler, DTO, Agent, or Web code.
- `NewChatModel` validates and normalizes the complete Lumilio configuration
  before calling a provider constructor. Provider constructors receive the
  selected key, model, endpoint, and any provider-local constants explicitly.
- No Lumilio runtime package calls `os.Getenv`, `os.LookupEnv`, `godotenv`, a
  cloud default-credential chain, or a provider singleton to configure an LLM.
  Tests poison well-known provider environment variables and prove missing
  settings still fail rather than being recovered from ambient state.
- The native DeepSeek `v0.1.7` dependency graph still contains
  `github.com/joho/godotenv` indirectly, and its underlying client can consult
  `DEEPSEEK_API_KEY` when handed an empty key. Therefore the current blanket
  module-name prohibition is replaced by a stronger behavior contract: an
  empty key is rejected before native adapter construction, and ambient values
  cannot make it valid. Do not remove the general architecture scan that bans
  direct environment reads from Lumilio runtime packages.
- Pin every direct module to the reviewed stable version in the upstream
  snapshot. Run `go mod tidy` in both `server/` and `desktop/`; the Desktop
  embeds Server through its local replace and must converge on the same module
  graph.
- Regenerate `desktop/licenses/THIRD_PARTY_NOTICES.txt` with
  `task desktop:licenses`. Never hand-edit the notices file.

## Fixed API and Web contract

- Extend the system-settings response with a generated, UI-safe supported
  provider descriptor list. Each entry exposes the stable provider ID and the
  two requirement booleans. It contains no secret, endpoint default, SDK type,
  model recommendation, or availability claim based on a network probe.
- Request DTOs accept provider strings and defer membership to the Server
  registry instead of maintaining `binding:"oneof=..."` lists that can drift.
  Unknown provider IDs remain rejected before persistence or construction.
- Apply the OpenAPI-first procedure and run `task dto`; do not hand-edit
  `web/src/lib/http-commons/schema.d.ts`.
- The Web builds its dropdown and required-field validation from the generated
  provider descriptors. It keeps a small exhaustive ID-to-localized-label map
  so an unknown future ID is not silently selectable without copy.
- Add the new provider names and any validation copy through the canonical
  bilingual terminology/i18n workflow. Do not render Server error strings or
  provider response bodies as UI copy. Coordinate the validation failure shape
  with the active `api-error-i18n` plan rather than creating a competing error
  envelope.
- Update the Settings feature's `doc.ts` and regenerate `doc.md`; never edit
  the generated Markdown by hand.

## Execution phases

### Phase 0 — Lock the gaps and compatibility baseline

- Add a Server provider-matrix contract around the existing settings validator
  and model factory. The target assertions for the four missing provider IDs
  and native DeepSeek identity should fail before the implementation.
- Add a poisoned-environment regression: populate the well-known key/model
  variables used by the selected upstream SDKs, pass incomplete Lumilio
  settings, and prove validation fails before adapter construction.
- Lock the current successful Ark, OpenAI, DeepSeek, and Ollama configuration
  requirements so the expansion cannot accidentally weaken secret clearing or
  endpoint validation.
- Lock the current ADK Agent lifecycle at the Lumilio seam: streamed text,
  streamed tool calls/results, token-usage side channel, summarization,
  cancellation, confirmation interrupt/checkpoint, resume, and session
  persistence. Use deterministic fake models and the existing stores; do not
  call public providers in the ordinary test suite.
- Record the current Server and Desktop module graphs and run the pre-upgrade
  relevant gates. Any pre-existing failure is separated from the upgrade
  before dependency edits begin.

### Phase 1 — Upgrade Eino core/ADK and existing adapters

- Pin Eino core/ADK to `v0.9.14`, Ark to `v0.1.69`, OpenAI to `v0.1.13`, and
  Ollama to `v0.1.9` in `server/go.mod`; tidy Server and Desktop modules.
- Resolve compile/API changes locally at the adapter and Agent seams. Do not
  change public Agent events, persisted conversation rows, checkpoint keys,
  or audit JSON to accommodate the upgrade.
- Run the Phase 0 Agent lifecycle suite with the upgraded ADK, paying special
  attention to summarization errors, stream reduction, cancellation, and
  checkpoint resume because those areas changed upstream after `v0.9.6`.
- Prove a checkpoint created by the pre-upgrade Lumilio path can either resume
  successfully on `v0.9.14` or is rejected through an explicit, recoverable
  application transition. Do not silently discard an awaiting-confirmation
  run.
- Land this as an independently reviewable dependency/compatibility slice
  before adding providers when practical.

### Phase 2 — Add the provider registry and native constructors

- Replace `IsSupportedLLMProvider` and scattered conditionals with the leaf
  provider descriptor registry and table-driven validation.
- Add the reviewed Claude, DeepSeek, Gemini, Qwen, and OpenRouter modules at the
  frozen tags. Keep all eight modules as direct Server requirements because
  product code imports them directly.
- Implement one constructor per provider behind the existing
  `model.ToolCallingChatModel` return type. Gemini explicitly builds its
  `genai.Client`; Claude explicitly selects direct Anthropic mode; DeepSeek
  passes the stored key/model/base URL to the native adapter.
- Delete the DeepSeek-through-OpenAI branch and its rationale. Change the
  architecture contract from a DeepSeek-module ban to the frozen
  pre-construction/no-ambient behavior checks.
- Add compile-time interface assertions and table-driven constructor tests for
  all eight providers. Use provider-shaped `httptest` fixtures to cover a
  normal response, a streamed response, and at least one tool call/result
  round trip for each adapter.
- Verify provider errors are logged/translated at the existing boundary and
  never place keys, Authorization headers, raw provider bodies, or full prompts
  in ordinary logs.

### Phase 3 — Publish the provider contract to Settings

- Add the supported-provider descriptor DTO and update the settings update and
  validation annotations to use domain validation.
- Follow `lumilio-api-contract-change`: regenerate Server OpenAPI, Web schema
  types, and ReDoc together with `task dto`.
- Update the AI settings draft to consume the Server descriptors for the
  dropdown, key requirement, and base-URL requirement. Preserve dirty/reset,
  stored-key ownership, explicit Validate, and Save/enable behavior.
- Follow `lumilio-frontend-i18n` for the Claude, Gemini, Qwen, and OpenRouter
  display names and any new human-facing validation strings; extract, fill
  English and Simplified Chinese, and run the terminology gates.
- Add integration coverage that selects every advertised provider, verifies
  the correct required fields, submits generated API payloads without casts,
  and never shows a provider that the Server did not advertise.
- Follow `lumilio-feature-doc`: update Settings `doc.ts` and regenerate its
  sibling `doc.md`.

### Phase 4 — Package and validate the complete matrix

- Tidy both Go modules again after all imports settle, review every direct and
  transitive dependency delta, and ensure Server/Desktop select exactly one
  Eino core version.
- Regenerate and review Desktop third-party notices. Confirm every new module's
  license is distributable under the existing packaging policy.
- Run a credentialed, opt-in smoke against one tool-capable model for each
  cloud provider and one local tool-capable Ollama model. Exercise Validate,
  one streamed answer, and one harmless read-only Agent tool call. Record only
  provider, model, date, and pass/fail; never record keys or sensitive prompts.
- Exercise switching from a keyed provider to another keyed provider, to
  Ollama, and to unset. Prove a prior encrypted key is not reused and existing
  in-flight runs keep their already-constructed model.
- Update `BACKEND.md` with the provider/configuration boundary. Reconcile any
  surviving deferred provider work into the tech-debt tracker only if it has a
  concrete owner and trigger.
- Follow `lumilio-select-checks` before claiming completion, extract durable
  decisions, and delete this active plan according to `lumilio-exec-plan`.

## Recommended PR slices

1. Baseline contracts plus Eino/ADK and existing-adapter upgrades.
2. Server provider registry, five new dependencies, native DeepSeek, and
   adapter conformance tests.
3. OpenAPI/settings Web contract, bilingual labels, feature docs, and generated
   artifacts.
4. Desktop module/notices reconciliation, credentialed smoke evidence, and
   plan completion memory.

Each PR updates `Status:` and the phase checklist below to match landed reality.
Do not mark a provider supported between the Server factory landing and the Web
contract/tests that make it safely configurable.

## Validation boundaries

The plan is complete only when all of the following are observable:

- the Server and Desktop module graphs select Eino `v0.9.14`; no
  `v0.10.0-alpha.*` module is present;
- the reviewed stable versions of all eight enabled provider adapters are
  direct Server dependencies, while Qianfan and all `agentic*` provider
  packages are absent;
- the provider registry, API response, generated TypeScript, Web dropdown, and
  validation UI expose exactly the same eight stable IDs and requirement
  flags;
- DeepSeek reports native DeepSeek component identity and passes native
  generate, stream, reasoning-content, and tool-call fixtures without using
  the OpenAI adapter;
- poisoned provider environment variables cannot satisfy, replace, or alter an
  incomplete stored configuration, and no provider change reuses the previous
  encrypted credential;
- every enabled adapter completes a deterministic tool-calling stream contract
  in Server tests, and every provider has a dated credentialed smoke record
  against a tool-capable model before release support is claimed;
- the pre-upgrade ADK lifecycle and checkpoint/resume contracts pass on
  `v0.9.14`, including cancellation, confirmation, summarization, persistence,
  token usage, and audit wrapping;
- `task server:test`, `task web:test`, `task desktop:test`,
  `task architecture:check`, and `task verify:generated` pass for the final
  diff, with narrower checks used on earlier slices as selected by
  `lumilio-select-checks`;
- `task desktop:licenses` is reproducible and leaves the reviewed notices file
  current; and
- the generated OpenAPI/types/docs, bilingual catalogs, `BACKEND.md`, Settings
  feature documentation, and the final durable decision record agree with the
  shipped provider boundary.

## Progress

- [x] Upstream stable Eino/ADK and classic provider support investigated and
  frozen (2026-08-17).
- [x] Lumilio target provider/configuration boundary frozen (2026-08-17).
- [ ] Phase 0 — lock gaps and compatibility baseline.
- [ ] Phase 1 — upgrade Eino core/ADK and existing adapters.
- [ ] Phase 2 — add registry, native DeepSeek, and new provider adapters.
- [ ] Phase 3 — publish the generated Settings/API/Web contract.
- [ ] Phase 4 — package, smoke, document, and complete the plan.

## Decisions (frozen)

- 2026-08-17: Pin the latest stable Eino `v0.9.14`; do not adopt the parallel
  `v0.10` alpha line.
- 2026-08-17: Treat Eino ADK as part of the Eino core pin. There is no separate
  ADK dependency version to upgrade.
- 2026-08-17: Retain the classic `schema.Message`/`ToolCallingChatModel` ADK
  path. Provider expansion does not earn an AgenticModel persistence migration.
- 2026-08-17: Enable Ark, OpenAI, DeepSeek, Ollama, Claude, Gemini, Qwen, and
  OpenRouter through official adapters; defer Qianfan until an instance-scoped,
  explicit credential contract exists.
- 2026-08-17: Replace the brittle native-DeepSeek module ban with validation
  before construction plus ambient-environment poisoning tests. An indirect
  package name is not the configuration boundary; observed runtime behavior is.
- 2026-08-17: Keep the existing single encrypted API-key aggregate. Hosted
  variants or providers needing multiple secrets require a future explicit
  schema and distinct provider identities.
- 2026-08-17: Make the Server descriptor registry authoritative and publish its
  safe requirement flags through OpenAPI so backend, generated types, and Web
  cannot drift as providers change.
