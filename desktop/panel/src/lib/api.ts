import type {
  CompletePayload,
  LogResult,
  LumenAction,
  LumenSavePayload,
  PanelState,
  PickResult,
  RepositoryIdentityConflict,
  RepositoryInfo,
  RuntimeConfigApplyRequest,
  RuntimeConfigApplyResult,
  RuntimeConfigPatchNetworkRequest,
  RuntimeConfigRequest,
  RuntimeConfigValidation,
  RuntimeConfigView,
  StorageLocation,
  StorageLocationIdentityConflict,
} from "./types.ts";

export class PanelAPIError extends Error {
  readonly status: number;
  readonly payload?: unknown;

  constructor(message: string, status: number, payload?: unknown) {
    super(message);
    this.status = status;
    this.payload = payload;
  }
}

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url);
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<T>;
}

async function postJSON<T>(url: string, body: unknown = {}): Promise<T> {
  const res = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const contentType = res.headers.get("content-type") ?? "";
    const payload = contentType.includes("application/json") ? await res.json() : await res.text();
    const message =
      typeof payload === "string"
        ? payload
        : typeof payload === "object" && payload && "message" in payload
          ? String(payload.message)
          : `${res.status} ${res.statusText}`;
    throw new PanelAPIError(message, res.status, payload);
  }
  return res.json() as Promise<T>;
}

export const api = {
  state: () => getJSON<PanelState>("/__onb/state"),
  pickStorage: () => postJSON<PickResult>("/__onb/pick"),
  pickCache: () => postJSON<PickResult>("/__onb/pick-cache"),
  complete: (payload: CompletePayload) => postJSON<{ ok: boolean }>("/__onb/complete", payload),
  saveRegion: (region: string) => postJSON<{ ok: boolean }>("/__onb/region", { region }),
  restartRuntime: () => postJSON<{ accepted: boolean }>("/__onb/runtime/restart"),
  runtimeConfig: () => getJSON<RuntimeConfigView>("/__onb/runtime/config"),
  validateRuntimeConfig: (payload: RuntimeConfigRequest) =>
    postJSON<RuntimeConfigValidation>("/__onb/runtime/config/validate", payload),
  patchRuntimeNetwork: (payload: RuntimeConfigPatchNetworkRequest) =>
    postJSON<RuntimeConfigValidation>("/__onb/runtime/config/patch-network", payload),
  applyRuntimeConfig: (payload: RuntimeConfigApplyRequest) =>
    postJSON<RuntimeConfigApplyResult>("/__onb/runtime/config/apply", payload),
  restoreRuntimeConfig: () => postJSON<{ accepted: boolean }>("/__onb/runtime/config/restore"),
  lumenSave: (payload: LumenSavePayload) => postJSON<{ ok: boolean }>("/__onb/lumen-save", payload),
  lumenAction: (action: LumenAction) =>
    postJSON<{ ok: boolean }>("/__onb/lumen-action", { action }),
  log: (source: string) => getJSON<LogResult>(`/__onb/log?source=${encodeURIComponent(source)}`),
  openPath: (path: string) => postJSON<{ ok: boolean }>("/__onb/open", { path }),
  openApp: () => postJSON<{ ok: boolean }>("/__onb/open-app"),
  storageLocations: () => getJSON<{ locations: StorageLocation[] }>("/__onb/storage-locations"),
  pickStorageLocation: () =>
    postJSON<{ cancelled?: boolean; location?: StorageLocation; warnings?: string[] }>(
      "/__onb/pick-storage-location",
    ),
  removeStorageLocation: (id: string) =>
    postJSON<{ ok: boolean }>("/__onb/remove-storage-location", { id }),
  resolveStorageLocationConflict: (conflict: StorageLocationIdentityConflict) =>
    postJSON<{ location: StorageLocation }>("/__onb/storage-location-conflict", {
      rootId: conflict.rootId,
      path: conflict.requestedPath,
    }),
  attachRepository: () =>
    postJSON<{ cancelled?: boolean; repository?: RepositoryInfo }>("/__onb/attach-repository"),
  resolveRepositoryConflict: (action: "relocate" | "copy", conflict: RepositoryIdentityConflict) =>
    postJSON<{ repository: RepositoryInfo }>("/__onb/repository-conflict", {
      action,
      repositoryId: conflict.repositoryId,
      path: conflict.requestedPath,
    }),
  legal: async (doc: "terms" | "license" | "third-party", lang: string): Promise<string> => {
    const url = doc === "terms" ? `/__onb/legal/terms?lang=${lang}` : `/__onb/legal/${doc}`;
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
    return res.text();
  },
};
