// Shapes of the desktop app's /__onb JSON API (see desktop/onboarding.go and
// desktop/control_panel.go). Field names must stay in sync with the Go side.

export interface Validation {
  reachable: boolean;
  writable: boolean;
  freeBytes?: number;
  freeHuman?: string;
}

export interface BackendChoice {
  name: string;
  profile: string;
  recommended?: boolean;
}

export interface Preset {
  name: string;
  minRamGB: number;
  minDiskGB: number;
}

export type LumenRunState =
  | ""
  | "installing"
  | "starting"
  | "checking"
  | "stopping"
  | "running"
  | "failed"
  | "off";

export interface DownloadStatus {
  model: string;
  file: string;
  bytesDone: number;
  bytesTotal: number;
  filesDone: number;
  filesTotal: number;
}

export interface LumenInfo {
  enabled: boolean;
  state: LumenRunState;
  error: string;
  preset: string;
  backend: string;
  profile: string;
  cacheDir: string;
  previousCacheDir: string;
  installedVersion: string;
  latestVersion: string;
  /** Control-plane phase reported by the hub itself (empty when not running). */
  phase?: string;
  download?: DownloadStatus | null;
}

export interface DashboardPaths {
  storage?: string;
  logs?: string;
  backups?: string;
  appData?: string;
}

export interface PanelState {
  mode: "onboarding" | "dashboard";
  lang: string;
  region: string;
  path: string;
  validation: Validation;
  version: string;
  tosRev: string;
  ready: boolean;
  serverURL: string;
  stage: string;
  paths: DashboardPaths;
  lumen: LumenInfo;
  backends: BackendChoice[];
  presets: Preset[];
  recommendedPreset: string;
  memoryGB: number;
  cacheValidation: Validation;
}

export interface PickResult {
  cancelled?: boolean;
  path?: string;
  validation?: Validation;
}

export interface LogResult {
  content: string;
  path: string;
}

export type LumenAction = "enable" | "disable" | "restart" | "check" | "update";

export interface CompletePayload {
  path: string;
  lang: string;
  region: string;
  agreed: boolean;
  enableLumen: boolean;
  preset: string;
  backend: string;
  profile: string;
  cacheDir: string;
}

export interface LumenSavePayload {
  preset: string;
  backend: string;
  profile: string;
  cacheDir: string;
}
