import type {
  CompletePayload,
  LogResult,
  LumenAction,
  LumenSavePayload,
  PanelState,
  PickResult,
} from "./types.ts";

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
  if (!res.ok) throw new Error(await res.text());
  return res.json() as Promise<T>;
}

export const api = {
  state: () => getJSON<PanelState>("/__onb/state"),
  pickStorage: () => postJSON<PickResult>("/__onb/pick"),
  pickCache: () => postJSON<PickResult>("/__onb/pick-cache"),
  complete: (payload: CompletePayload) => postJSON<{ ok: boolean }>("/__onb/complete", payload),
  saveRegion: (region: string) => postJSON<{ ok: boolean }>("/__onb/region", { region }),
  lumenSave: (payload: LumenSavePayload) => postJSON<{ ok: boolean }>("/__onb/lumen-save", payload),
  lumenAction: (action: LumenAction) =>
    postJSON<{ ok: boolean }>("/__onb/lumen-action", { action }),
  log: (source: string) => getJSON<LogResult>(`/__onb/log?source=${encodeURIComponent(source)}`),
  openPath: (path: string) => postJSON<{ ok: boolean }>("/__onb/open", { path }),
  openApp: () => postJSON<{ ok: boolean }>("/__onb/open-app"),
  legal: async (doc: "terms" | "license" | "third-party", lang: string): Promise<string> => {
    const url = doc === "terms" ? `/__onb/legal/terms?lang=${lang}` : `/__onb/legal/${doc}`;
    const res = await fetch(url);
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
    return res.text();
  },
};
