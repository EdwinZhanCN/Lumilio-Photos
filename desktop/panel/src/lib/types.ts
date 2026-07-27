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
  serverConfig?: string;
}

export type NetworkMode = "local" | "lan_http" | "external_https";

export interface NetworkInfo {
  mode: NetworkMode;
  primaryOrigin: string;
  listen: string;
  trustedProxyCIDRs: string[];
  lanWarningAcceptedVersion: number;
  lanAddresses: string[];
}

export type RuntimePhase = "stopped" | "starting" | "running" | "restarting" | "failed";

export interface RuntimeNetworkSummary {
  mode: NetworkMode;
  listen: string;
  primaryOrigin: string;
  tlsMode: "off" | "external";
  proxyMode: "disabled" | "required";
  trustedProxyCIDRs: string[];
  passkeyOrigin: string;
  rpID: string;
  passkeyEnabled: boolean;
  remotePasskeyAvailable: boolean;
}

export interface RuntimeSnapshot {
  phase: RuntimePhase;
  stage?: string;
  errorCode?: string;
  errorMessage?: string;
  browserURL?: string;
  canOpen: boolean;
  canRestart: boolean;
  lastKnownGoodAvailable: boolean;
  network: RuntimeNetworkSummary;
  operationActive: boolean;
}

export interface ConfigIssue {
  field?: string;
  code: string;
  message: string;
}

export interface SemanticChange {
  field: string;
  before: string;
  after: string;
}

export interface RuntimeConfigView {
  currentToml: string;
  candidateToml: string;
  baseFingerprint: string;
  lastKnownGoodAvailable: boolean;
  hostManagedPaths: string[];
  network: RuntimeNetworkSummary;
  issues: ConfigIssue[];
  semanticChanges: SemanticChange[];
}

export interface RuntimeConfigValidation {
  valid: boolean;
  candidateToml: string;
  baseFingerprint: string;
  network: RuntimeNetworkSummary;
  issues: ConfigIssue[];
  semanticChanges: SemanticChange[];
  requiresRestart: boolean;
}

export interface PanelState {
  mode: "onboarding" | "dashboard";
  lang: string;
  region: string;
  path: string;
  validation: Validation;
  version: string;
  tosRev: string;
  runtime: RuntimeSnapshot;
  paths: DashboardPaths;
  networkHost: Pick<NetworkInfo, "lanWarningAcceptedVersion" | "lanAddresses">;
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

export interface StorageLocation {
  id: string;
  name: string;
  path: string;
  kind: "default" | "external";
  status: "active" | "offline" | "error";
}

export interface RepositoryInfo {
  id: string;
  name: string;
  path: string;
  status: string;
}

export interface RepositoryIdentityConflict {
  repositoryId: string;
  registeredPath: string;
  requestedPath: string;
  actions: Array<"relocate" | "copy">;
}

export interface StorageLocationIdentityConflict {
  rootId: string;
  registeredPath: string;
  requestedPath: string;
  actions: Array<"relocate">;
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

export interface NetworkCandidateInput {
  mode: NetworkMode;
  primaryOrigin: string;
  listen: string;
  proxyLocation: "same_host" | "remote";
  trustedProxyCIDRs: string[];
  acceptLANWarning: boolean;
}

export interface RuntimeConfigRequest {
  baseFingerprint: string;
  toml: string;
}

export interface RuntimeConfigPatchNetworkRequest extends RuntimeConfigRequest {
  network: NetworkCandidateInput;
}

export interface RuntimeConfigApplyRequest extends RuntimeConfigRequest {
  acceptLANWarning?: boolean;
}

export interface RuntimeConfigApplyResult {
  accepted: boolean;
  validation: RuntimeConfigValidation;
}
