import type React from "react";

export type StudioPluginPanel = "frames" | "develop";

export interface RuntimeManifestMount {
  panel: StudioPluginPanel;
  order?: number;
}

export interface RuntimeManifestEntries {
  ui: string;
  runner: string;
}

export interface RuntimeManifestCompatibility {
  studioApi: string;
  minHostVersion?: string;
  maxHostVersion?: string;
}

export interface RuntimeManifestSignature {
  keyId: string;
  algorithm: "ECDSA_P256_SHA256";
  value: string;
}

export interface RuntimeManifestV1 {
  schemaVersion: 1;
  id: string;
  version: string;
  displayName: string;
  description?: string;
  mount: RuntimeManifestMount;
  entries: RuntimeManifestEntries;
  permissions: string[];
  compatibility: RuntimeManifestCompatibility;
  signature: RuntimeManifestSignature;
}

export interface PluginRunResult {
  bytes: Uint8Array;
  mimeType: string;
  fileName: string;
}

export interface StudioPluginPanelProps {
  value: Record<string, unknown>;
  onChange: (next: Record<string, unknown>) => void;
  disabled?: boolean;
}

export interface StudioPluginUiMeta {
  id: string;
  version: string;
  displayName: string;
  mount: RuntimeManifestMount;
}

export interface StudioPluginUiModule {
  meta: StudioPluginUiMeta;
  defaultParams: Record<string, unknown>;
  Panel: React.ComponentType<StudioPluginPanelProps>;
  normalizeParams?: (raw: Record<string, unknown>) => Record<string, unknown>;
}

export interface StudioPluginRunnerContext {
  inputFile: File;
  signal: AbortSignal;
  manifest: RuntimeManifestV1;
}

export interface StudioPluginRunnerHelpers {
  reportProgress?: (processed: number, total: number) => void;
}

export interface StudioPluginRunnerModule {
  run: (
    ctx: StudioPluginRunnerContext,
    params: Record<string, unknown>,
    helpers?: StudioPluginRunnerHelpers,
  ) => Promise<PluginRunResult>;
}

export interface CatalogPluginSummary {
  id: string;
  displayName: string;
  description?: string;
  panel: StudioPluginPanel;
  latestVersion: string;
}

export interface PluginRevocationRecord {
  id: string;
  version: string;
  reason?: string;
  active: boolean;
}

export interface InstalledPluginRecord {
  pluginId: string;
  version: string;
  installedAt: string;
}
