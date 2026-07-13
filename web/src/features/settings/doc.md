# Settings

The settings feature owns the authenticated settings route, local
preferences, server-backed system settings drafts, runtime info display, AI
and cloud admin tabs, user-management surface, and the repository scope hooks
shared by other features. It is a boundary feature: pages such as Assets,
Collections, Home, Upload, Monitor, and Lumilio consume the hooks here but do
not own their persistence rules.

## State

Client-only preferences live in [usePreferencesStore](./preferences.ts), persisted under
[PREFERENCES_STORAGE_KEY](./settings.registry.ts). [usePreference](./preferences.ts) applies immediately,
while [useDebouncedPreference](./preferences.ts) keeps high-frequency controls such as
health-check intervals and gallery columns responsive before writing to
localStorage.

Server-backed settings use the shared [useDraftSettings](./hooks/useDraftSettings.ts) contract:
tabs edit a local draft, expose dirty/reset/save state through
[SettingsSaveBar](./components/renew/SettingsSaveBar.tsx), and commit through [useUpdateSystemSettings](./hooks/useSystemSettings.ts).
[useAISettingsDraft](./hooks/useAISettingsDraft.ts) is the current rich draft adapter for LLM/ML
settings, including API-key clearing semantics and server normalization.

Repository preference state is deliberately split:

- [useBrowseScope](./hooks/useBrowseScope.ts) answers "what repository am I looking at?" for list
  pages. "All repositories" is a valid empty preference.
- [useWorkingRepository](./hooks/useWorkingRepository.ts) answers "where should new content land?" for
  upload. It resolves to a concrete repository, falling back to primary/first
  repository once repository options load.

Both repository IDs are user-scoped session state. Authentication reset
clears them while retaining device-level language, theme, and layout choices.

## Data

[useSystemSettings](./hooks/useSystemSettings.ts) reads `/api/v1/settings/system`; mutations go
through [useUpdateSystemSettings](./hooks/useSystemSettings.ts), which invalidates setup status and
capabilities because system settings affect bootstrap gates and AI runtime
availability. [useValidateLLMSettings](./hooks/useSystemSettings.ts) validates the saved LLM config
as an explicit action instead of on every keystroke.

[useRuntimeInfo](./hooks/useRuntimeInfo.ts) reads `/api/v1/settings/runtime-info` for effective
TOML/env-derived runtime configuration. It is display-only in the UI; users
change those values outside the SPA and restart the server.

[useIndexingRepositories](./hooks/useAssetIndexing.ts) is the shared repository option source used
by browse/working scope pickers and repository-aware status surfaces.
Cloud credentials and repository imports live behind [useCloudProviders](./hooks/useCloudSync.ts),
[useCloudCredentials](./hooks/useCloudSync.ts), [useCreateCloudCredential](./hooks/useCloudSync.ts), and the
repository cloud hooks in `useCloudSync.ts`.

## Composition

```mermaid
flowchart TD
    ROUTE["/settings"] --> HEADER["PageHeader"]
    ROUTE --> SHELL["SettingsShell"]
    SHELL --> URL["?tab=..."]
    SHELL --> ACCOUNT["AccountTab"]
    SHELL --> APPEAR["AppearanceTab"]
    SHELL --> SERVER["ServerTab"]
    SHELL --> ABOUT["AboutTab"]
    SHELL -. admin .-> AI["AiTab"]
    SHELL -. admin .-> CLOUD["CloudTab"]
    SHELL -. admin .-> USERS["UsersTab"]
    APPEAR --> PREFS["usePreferencesStore"]
    SERVER --> RUNTIME["useRuntimeInfo"]
    AI --> DRAFT["useAISettingsDraft"]
    CLOUD --> CLOUDAPI["cloud hooks"]
```

[Settings](./routes/Settings.tsx) renders the route header and delegates the tabbed surface to
[SettingsShell](./components/renew/SettingsShell.tsx). The shell always shows [AccountTab](./components/renew/tabs/AccountTab.tsx),
[AppearanceTab](./components/renew/tabs/AppearanceTab.tsx), [ServerTab](./components/renew/tabs/ServerTab.tsx), and [AboutTab](./components/renew/tabs/AboutTab.tsx); admin users additionally see
[AiTab](./components/renew/tabs/AiTab.tsx), [CloudTab](./components/renew/tabs/CloudTab.tsx), and [UsersTab](./components/renew/tabs/UsersTab.tsx). The visual hierarchy is
centralized in [SettingsPage](./components/renew/SettingsPage.tsx), [SettingsGroup](./components/renew/SettingsGroup.tsx),
[SettingsRow](./components/renew/SettingsGroup.tsx), and [SettingsBlock](./components/renew/SettingsGroup.tsx); tabs should compose those
primitives instead of inventing local section chrome.

## Decisions

Settings distinguishes instant preferences from manual-save system settings.
Preferences are user-local and reversible through localStorage; system
settings are shared backend state and need explicit Save/Reset affordances.

Runtime defaults do not live here. The Server tab reports effective runtime
config, but durable server defaults belong in TOML and backend config code.

Browse scope and working repository must not be collapsed into one setting:
list pages can intentionally show all repositories, while upload must always
resolve to one concrete target.
