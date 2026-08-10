import type { HostActionKind } from "../api/useNativeHostActions";

function storageKey(kind: HostActionKind, scope: string): string {
  return `lumilio.hostAction.${kind}.${scope || "new"}`;
}

export function loadNativeHostActionID(kind: HostActionKind, scope: string): string | null {
  return window.sessionStorage.getItem(storageKey(kind, scope));
}

export function saveNativeHostActionID(
  kind: HostActionKind,
  scope: string,
  actionID: string,
): void {
  window.sessionStorage.setItem(storageKey(kind, scope), actionID);
}

export function clearNativeHostActionID(kind: HostActionKind, scope: string): void {
  window.sessionStorage.removeItem(storageKey(kind, scope));
}
