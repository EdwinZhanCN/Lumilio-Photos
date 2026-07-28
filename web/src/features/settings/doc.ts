/**
 * # Settings
 *
 * Settings owns the authenticated `/settings` route, device-local preferences,
 * server-backed mutable system settings, runtime information, SQLite backup
 * management, cloud-account composition, and administrator user management.
 * Repository and Cloud data contracts remain in their own features.
 *
 * ## State
 *
 * Device-local preferences live in the lower shared
 * {@link usePreferencesStore} under {@link PREFERENCES_STORAGE_KEY}.
 * {@link usePreference} applies ordinary choices immediately, while
 * {@link useDebouncedPreference} delays high-frequency persistence.
 *
 * Rich server-backed editors use {@link useDraftSettings}: a local draft,
 * dirty/reset/save state, and explicit commit through
 * {@link SettingsSaveBar}. {@link useAISettingsDraft} adapts LLM credentials
 * and semantic, video-semantic, BioCLIP, OCR, and face switches. Server facts
 * remain Query data and are not copied into the preferences store.
 *
 * The active settings tab is the `tab` URL parameter. Repository browse and
 * upload preferences are owned by Repositories; authentication reset clears
 * those user-scoped ids while retaining device language, theme, and layout.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/settings"] --> SHELL["SettingsShell"]
 *     SHELL --> ACCOUNT["Account"]
 *     SHELL --> APPEARANCE["Appearance"]
 *     SHELL --> CLOUD["Cloud"]
 *     SHELL --> SERVER["Server + backups"]
 *     SHELL --> ABOUT["About"]
 *     SHELL -. admin .-> AI["AI"]
 *     SHELL -. admin .-> USERS["Users"]
 *     AI --> SAVE["SettingsSaveBar"]
 * ```
 *
 * {@link Settings} delegates tab composition to {@link SettingsShell}. Account,
 * Appearance, Cloud, Server, and About are available to authenticated users;
 * AI and Users are admin-only. Tabs share {@link SettingsPage},
 * {@link SettingsGroup}, and {@link SettingsRow} rather than inventing section
 * chrome.
 *
 * {@link BackupSection} owns automatic backup schedule/retention plus list,
 * create, download, restore, and delete interaction. A successful restore
 * reloads the application because the entire catalog and every cached server
 * fact have changed.
 *
 * ## Data
 *
 * {@link useSystemSettings} reads `/api/v1/settings/system`.
 * {@link useUpdateSystemSettings} invalidates system settings, setup status,
 * and capabilities; {@link useValidateLLMSettings} is an explicit validation
 * command. {@link useRuntimeInfo} reports effective manifest-derived runtime
 * configuration and is display-only.
 *
 * {@link useBackups} owns the backup list and temporary post-create polling;
 * {@link useRestoreBackup} performs catalog replacement. Cloud tabs consume
 * the Cloud public entry, and user/account tabs consume Users/Auth public
 * entries. The Settings root `index.ts` exposes only preference effects and
 * the narrow preference hook required by application composition.
 *
 * @module
 */
import type { useBackups, useRestoreBackup } from "./api/useBackups.ts";
import type { useRuntimeInfo } from "./api/useRuntimeInfo.ts";
import type {
  useSystemSettings,
  useUpdateSystemSettings,
  useValidateLLMSettings,
} from "./api/useSystemSettings.ts";
import type { SettingsGroup, SettingsRow } from "./components/SettingsGroup.tsx";
import type { SettingsPage } from "./components/SettingsPage.tsx";
import type { SettingsSaveBar } from "./components/SettingsSaveBar.tsx";
import type { useAISettingsDraft } from "./flows/ai/useAISettingsDraft.ts";
import type BackupSection from "./flows/server/BackupSection.tsx";
import type Settings from "./flows/shell/SettingsPageFlow.tsx";
import type SettingsShell from "./flows/shell/SettingsShell.tsx";
import type { useDraftSettings } from "./hooks/useDraftSettings.ts";
import type { PREFERENCES_STORAGE_KEY } from "./state/registry.ts";
import type {
  useDebouncedPreference,
  usePreference,
  usePreferencesStore,
} from "../../lib/preferences/preferences.ts";

export {};
