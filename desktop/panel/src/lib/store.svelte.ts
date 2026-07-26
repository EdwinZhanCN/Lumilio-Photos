import { api } from "./api.ts";
import { setLang } from "./i18n.svelte.ts";
import type { PanelState } from "./types.ts";

let initialized = false;

export const store = $state({
  data: null as PanelState | null,
  error: "",
});

export async function refreshState(): Promise<void> {
  try {
    const d = await api.state();
    if (!initialized) {
      // The backend knows the persisted/OS language; only seed it once so a
      // user's in-session toggle is not overwritten by polling.
      setLang(d.lang);
      initialized = true;
    }
    store.data = d;
    store.error = "";
  } catch (e) {
    store.error = String(e);
  }
}

/** Switch to dashboard locally after the wizard completes (the backend flips
 * `mode` on its side too; this avoids waiting one poll cycle). */
export function enterDashboard(): void {
  if (store.data) store.data.mode = "dashboard";
}

export type ServiceStatus = "running" | "starting" | "restarting" | "off" | "failed" | "disabled";

const busyStates = new Set(["installing", "starting", "checking", "stopping"]);

export function photosStatus(d: PanelState): ServiceStatus {
  switch (d.runtime.phase) {
    case "running":
      return "running";
    case "failed":
      return "failed";
    case "stopped":
      return "off";
    case "restarting":
      return "restarting";
    default:
      return "starting";
  }
}

export function runtimeBusy(d: PanelState): boolean {
  return (
    d.runtime.phase === "starting" || d.runtime.phase === "restarting" || d.runtime.operationActive
  );
}

export function runtimeRunning(d: PanelState): boolean {
  return d.runtime.phase === "running";
}

export function runtimeFailed(d: PanelState): boolean {
  return d.runtime.phase === "failed";
}

export function canOpenLumilio(d: PanelState): boolean {
  return runtimeRunning(d) && d.runtime.canOpen && Boolean(d.runtime.browserURL);
}

export function canRestartRuntime(d: PanelState): boolean {
  return d.runtime.canRestart && !runtimeBusy(d);
}

export function hubStatus(d: PanelState): ServiceStatus {
  const l = d.lumen;
  if (l.state === "failed") return "failed";
  if (busyStates.has(l.state)) return "starting";
  if (l.state === "running") return "running";
  // Not running: distinguish "user turned it off" from a plain off state.
  return l.enabled ? "off" : "disabled";
}

export function anyServiceBusy(d: PanelState): boolean {
  return runtimeBusy(d) || busyStates.has(d.lumen.state);
}

export function hubUpdateAvailable(d: PanelState): boolean {
  const l = d.lumen;
  return Boolean(l.latestVersion && l.installedVersion && l.latestVersion !== l.installedVersion);
}
