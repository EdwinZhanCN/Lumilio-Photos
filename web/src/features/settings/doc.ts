/**
 * # Settings
 *
 * The settings feature owns the authenticated settings route, local
 * preferences, server-backed system settings drafts, runtime info display, AI
 * and cloud admin tabs, and the user-management surface. Repository scope and
 * cloud data access live in their own feature boundaries; Settings composes
 * those capabilities without owning their persistence or query rules.
 *
 * ## State
 *
 * Client-only preferences live in the lower shared {@link usePreferencesStore}
 * so theme effects do not depend on Settings UI. Settings keeps the public
 * preference API and the model remains persisted under
 * {@link PREFERENCES_STORAGE_KEY}. {@link usePreference} applies immediately,
 * while {@link useDebouncedPreference} keeps high-frequency controls such as
 * health-check intervals and gallery columns responsive before writing to
 * localStorage.
 *
 * Server-backed settings use the shared {@link useDraftSettings} contract:
 * tabs edit a local draft, expose dirty/reset/save state through
 * {@link SettingsSaveBar}, and commit through {@link useUpdateSystemSettings}.
 * {@link useAISettingsDraft} is the current rich draft adapter for LLM/ML
 * settings, including API-key clearing semantics and server normalization.
 *
 * Repository preference state is deliberately split by the Repositories feature:
 *
 * - {@link useBrowseScope} answers "what repository am I looking at?" for list
 *   pages. "All repositories" is a valid empty preference.
 * - {@link useWorkingRepository} answers "where should new content land?" for
 *   upload. It resolves to a concrete repository, falling back to primary/first
 *   repository once repository options load.
 *
 * Both repository IDs are user-scoped session state. Authentication reset
 * clears them while retaining device-level language, theme, and layout choices.
 *
 * ## Data
 *
 * {@link useSystemSettings} reads `/api/v1/settings/system`; mutations go
 * through {@link useUpdateSystemSettings}, which invalidates setup status and
 * capabilities because system settings affect bootstrap gates and AI runtime
 * availability. {@link useValidateLLMSettings} validates the saved LLM config
 * as an explicit action instead of on every keystroke.
 *
 * {@link useRuntimeInfo} reads `/api/v1/settings/runtime-info` for effective
 * TOML/env-derived runtime configuration. It is display-only in the UI; users
 * change those values outside the SPA and restart the server.
 *
 * {@link useRepositoryOptions} is the shared repository option source used
 * by browse/working scope pickers and repository-aware status surfaces.
 * Cloud credentials and repository imports live behind {@link useCloudProviders},
 * {@link useCloudCredentials}, {@link useCreateCloudCredential}, and the
 * repository cloud hooks exposed by the Cloud feature.
 *
 * ## Composition
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/settings"] --> HEADER["PageHeader"]
 *     ROUTE --> SHELL["SettingsShell"]
 *     SHELL --> URL["?tab=..."]
 *     SHELL --> ACCOUNT["AccountTab"]
 *     SHELL --> APPEAR["AppearanceTab"]
 *     SHELL --> SERVER["ServerTab"]
 *     SHELL --> ABOUT["AboutTab"]
 *     SHELL -. admin .-> AI["AiTab"]
 *     SHELL -. admin .-> CLOUD["CloudTab"]
 *     SHELL -. admin .-> USERS["UsersTab"]
 *     APPEAR --> PREFS["usePreferencesStore"]
 *     SERVER --> RUNTIME["useRuntimeInfo"]
 *     AI --> DRAFT["useAISettingsDraft"]
 *     CLOUD --> CLOUDAPI["cloud hooks"]
 * ```
 *
 * {@link Settings} renders the route header and delegates the tabbed surface to
 * {@link SettingsShell}. The shell always shows {@link AccountTab},
 * {@link AppearanceTab}, {@link ServerTab}, and {@link AboutTab}; admin users additionally see
 * {@link AiTab}, {@link CloudTab}, and {@link UsersTab}. The visual hierarchy is
 * centralized in {@link SettingsPage}, {@link SettingsGroup},
 * {@link SettingsRow}, and {@link SettingsBlock}; tabs should compose those
 * primitives instead of inventing local section chrome.
 *
 * ## Decisions
 *
 * Settings distinguishes instant preferences from manual-save system settings.
 * Preferences are user-local and reversible through localStorage; system
 * settings are shared backend state and need explicit Save/Reset affordances.
 *
 * Runtime defaults do not live here. The Server tab reports effective runtime
 * config, but durable server defaults belong in TOML and backend config code.
 *
 * Browse scope and working repository must not be collapsed into one setting:
 * list pages can intentionally show all repositories, while upload must always
 * resolve to one concrete target.
 *
 * @module
 */
import type Settings from "./routes/Settings.tsx";
import type SettingsShell from "./components/SettingsShell.tsx";
import type { SettingsPage } from "./components/SettingsPage.tsx";
import type { SettingsBlock, SettingsGroup, SettingsRow } from "./components/SettingsGroup.tsx";
import type { SettingsSaveBar } from "./components/SettingsSaveBar.tsx";
import type AccountTab from "./components/tabs/AccountTab.tsx";
import type AppearanceTab from "./components/tabs/AppearanceTab.tsx";
import type AiTab from "./components/tabs/AiTab.tsx";
import type CloudTab from "./components/tabs/CloudTab.tsx";
import type ServerTab from "./components/tabs/ServerTab.tsx";
import type UsersTab from "./components/tabs/UsersTab.tsx";
import type AboutTab from "./components/tabs/AboutTab.tsx";
import type { useAISettingsDraft } from "./hooks/useAISettingsDraft.ts";
import type {
  useBrowseScope,
  useRepositoryOptions,
  useWorkingRepository,
} from "@/features/repositories";
import type {
  useCloudCredentials,
  useCloudProviders,
  useCreateCloudCredential,
} from "@/features/cloud";
import type { useDraftSettings } from "./hooks/useDraftSettings.ts";
import type { useRuntimeInfo } from "./api/useRuntimeInfo.ts";
import type {
  useSystemSettings,
  useUpdateSystemSettings,
  useValidateLLMSettings,
} from "./api/useSystemSettings.ts";
import type {
  useDebouncedPreference,
  usePreference,
  usePreferencesStore,
} from "../../lib/preferences/preferences.ts";
import type { PREFERENCES_STORAGE_KEY } from "./state/registry.ts";

export {};
