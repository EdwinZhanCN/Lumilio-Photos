import { spawnSync } from "node:child_process";
import { readFileSync } from "node:fs";
import type { components } from "../../src/lib/http-commons/schema.d.ts";
import { api, baseURL } from "./api";
import { compose, docker, repositoryRoot } from "./docker.ts";
import type { Workspace } from "./workspace";

export const AGENT_FIXTURE_MODEL = "lumilio-agent-e2e-v1";
export const AGENT_PLAIN_RESPONSE = "Deterministic Agent runtime response.";
export const AGENT_CONFIRMATION_RESPONSE = "Deterministic album update completed.";
export const AGENT_REJECTION_RESPONSE = "Deterministic album update declined.";
export const AGENT_COMMITTED_RECEIPT = "Added 1 photos to album";
export const AGENT_REJECTED_RECEIPT = "Album update was not applied: the user declined.";
export const AGENT_OCR_RESPONSE = "Stored OCR: Lumilio OCR first line / Lumilio OCR second line.";
export const AGENT_PROVIDER_PRIVATE_MARKER = "fixture-upstream-private-marker";

const scenarioPrefix = "LUMILIO_E2E_SCENARIO:";
const metricsURL =
  process.env.LUMILIO_E2E_AGENT_MODEL_METRICS_URL ?? "http://127.0.0.1:16659/metrics";
type Scenario =
  | { name: "plain" | "slow-stream" | "provider-error" | "read-ocr" }
  | { name: "confirm-add-to-album"; filename: string; album_title: string };

type BrowseResponse = components["schemas"]["dto.QueryAssetsResponseDTO"];
type Album = components["schemas"]["dto.GetAlbumResponseDTO"];
type AlbumAssets = components["schemas"]["dto.AlbumAssetsResponseDTO"];
type User = components["schemas"]["dto.UserDTO"];
type AssetDetail = components["schemas"]["dto.AssetDetailDTO"];

export type AgentModelMetrics = {
  requests_total: number;
  plain_completed: number;
  confirmation_lookup: number;
  confirmation_filter: number;
  confirmation_add: number;
  confirmation_final: number;
  confirmation_rejected: number;
  read_ocr_call: number;
  read_ocr_final: number;
  slow_started: number;
  slow_cancelled: number;
  provider_errors: number;
  auth_rejections: number;
  protocol_errors: number;
};

export type AgentAlbumFixture = {
  albumId: number;
  albumTitle: string;
  assetId: string;
  filename: string;
};

export type AgentOCRFixture = {
  assetId: string;
  filename: string;
  lines: string[];
};

export type AgentStreamFacts = {
  threadId: string;
  runId: string;
  interruptId?: string;
  effectId?: string;
};

export function agentScenarioPrompt(scenario: Scenario): string {
  return `${scenarioPrefix}${JSON.stringify(scenario)}`;
}

export async function agentModelMetrics(): Promise<AgentModelMetrics> {
  const response = await fetch(metricsURL);
  if (!response.ok) {
    throw new Error(`GET agent model metrics: ${response.status}`);
  }
  return (await response.json()) as AgentModelMetrics;
}

export async function agentAPIResponse(
  token: string,
  pathname: string,
  init: { method?: string; body?: unknown } = {},
): Promise<Response> {
  return fetch(`${baseURL}${pathname}`, {
    method: init.method,
    headers: {
      ...(init.body === undefined ? {} : { "content-type": "application/json" }),
      authorization: `Bearer ${token}`,
    },
    body: init.body === undefined ? undefined : JSON.stringify(init.body),
  });
}

export function agentStreamFacts(body: string): AgentStreamFacts {
  let threadId = "";
  let runId = "";
  let interruptId: string | undefined;
  let effectId: string | undefined;
  for (const frame of body.split(/\r?\n\r?\n/)) {
    const dataLine = frame
      .split(/\r?\n/)
      .find((line) => line.startsWith("data: "))
      ?.slice("data: ".length);
    if (!dataLine) continue;
    let data: unknown;
    try {
      data = JSON.parse(dataLine);
    } catch {
      continue;
    }
    if (!data || typeof data !== "object") continue;
    const record = data as Record<string, unknown>;
    if (typeof record.thread_id === "string" && typeof record.run_id === "string") {
      threadId = record.thread_id;
      runId = record.run_id;
    }
    const action = record.action as Record<string, unknown> | undefined;
    const interrupted = (action?.interrupted ?? action?.Interrupted) as
      | Record<string, unknown>
      | undefined;
    const contexts = interrupted?.InterruptContexts;
    if (!Array.isArray(contexts)) continue;
    const root = contexts.find(
      (candidate) =>
        candidate &&
        typeof candidate === "object" &&
        (candidate as Record<string, unknown>).IsRootCause === true,
    ) as Record<string, unknown> | undefined;
    const info = root?.Info as Record<string, unknown> | undefined;
    if (typeof root?.ID === "string") interruptId = root.ID;
    const candidateEffect = info?.effect_id ?? info?.EffectID;
    if (typeof candidateEffect === "string") effectId = candidateEffect;
  }
  if (!threadId || !runId) throw new Error("Agent SSE did not contain session_info identities");
  return { threadId, runId, interruptId, effectId };
}

export function agentStreamOutput(body: string): string {
  let output = "";
  for (const frame of body.split(/\r?\n\r?\n/)) {
    const dataLine = frame
      .split(/\r?\n/)
      .find((line) => line.startsWith("data: "))
      ?.slice("data: ".length);
    if (!dataLine) continue;
    try {
      const data = JSON.parse(dataLine) as Record<string, unknown>;
      if (typeof data.output === "string") output += data.output;
    } catch {
      // Non-JSON frames cannot contribute public assistant output.
    }
  }
  return output;
}

export function e2eServerLogsSince(since: Date): string {
  const result = spawnSync(
    "docker",
    [...docker, ...compose, "logs", "--since", since.toISOString(), "lumilio"],
    { cwd: repositoryRoot, encoding: "utf8" },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`docker compose logs failed (${String(result.status)}): ${result.stderr}`);
  }
  return `${result.stdout}${result.stderr}`;
}

export async function prepareAgentAlbumFixture(
  workspace: Workspace,
  identity: string,
): Promise<AgentAlbumFixture> {
  const filename = `agent-runtime-${workspace.username}-${identity}.jpg`;
  await uploadOwnedAsset(workspace, filename);
  const user = await api<User>("/api/v1/auth/me", { token: workspace.token });
  if (!user.user_id) throw new Error("agent runtime worker user has no id");

  let assetId = "";
  await poll(async () => {
    const response = await api<BrowseResponse>("/api/v1/assets/list", {
      method: "POST",
      token: workspace.token,
      body: JSON.stringify({
        query: filename,
        search_type: "filename",
        filter: { repository_id: workspace.repositoryId },
        pagination: { limit: 20, offset: 0 },
        stack_mode: "expanded",
      }),
    });
    const asset = response.items
      ?.map((item) => item.media_item?.primary_asset)
      .find((candidate) => candidate?.original_filename === filename);
    if (asset?.owner_id !== user.user_id) {
      throw new Error(
        `agent runtime asset owner ${String(asset?.owner_id)} did not match worker ${user.user_id}`,
      );
    }
    assetId = asset?.asset_id ?? "";
    return Boolean(assetId);
  });

  const albumTitle = `Agent Runtime ${workspace.username} ${identity}`;
  const album = await api<Album>("/api/v1/albums", {
    method: "POST",
    token: workspace.token,
    body: JSON.stringify({ album_name: albumTitle }),
  });
  if (!album.album_id) throw new Error("agent runtime fixture album has no id");
  const assets = await albumAssets(workspace.token, album.album_id);
  if (assets.count !== 0) throw new Error("agent runtime fixture album was not empty");

  return {
    albumId: album.album_id,
    albumTitle,
    assetId,
    filename,
  };
}

export async function prepareAgentOCRFixture(
  workspace: Workspace,
  identity: string,
): Promise<AgentOCRFixture> {
  const filename = `agent-ocr-${workspace.username}-${identity}.jpg`;
  await setOCREnabled(workspace.token, true);
  try {
    await uploadOwnedAsset(workspace, filename);
    const user = await api<User>("/api/v1/auth/me", { token: workspace.token });
    if (!user.user_id) throw new Error("agent OCR runtime worker user has no id");

    let assetId = "";
    await poll(async () => {
      const response = await api<BrowseResponse>("/api/v1/assets/list", {
        method: "POST",
        token: workspace.token,
        body: JSON.stringify({
          query: filename,
          search_type: "filename",
          filter: { repository_id: workspace.repositoryId },
          pagination: { limit: 20, offset: 0 },
          stack_mode: "expanded",
        }),
      });
      const asset = response.items
        ?.map((item) => item.media_item?.primary_asset)
        .find((candidate) => candidate?.original_filename === filename);
      if (asset?.owner_id !== user.user_id) {
        throw new Error(
          `agent OCR asset owner ${String(asset?.owner_id)} did not match worker ${user.user_id}`,
        );
      }
      assetId = asset?.asset_id ?? "";
      return Boolean(assetId);
    });

    const lines = ["Lumilio OCR first line", "Lumilio OCR second line"];
    await poll(async () => {
      const detail = await api<AssetDetail>(
        `/api/v1/assets/${assetId}?include_albums=false&include_faces=false&include_ocr=true&include_species=false&include_tags=false&include_thumbnails=false`,
        { token: workspace.token },
      );
      const actual = detail.ocr_result?.text_items?.map((item) => item.text_content) ?? [];
      return actual.length === lines.length && actual.every((line, index) => line === lines[index]);
    });

    return { assetId, filename, lines };
  } finally {
    await setOCREnabled(workspace.token, false);
  }
}

export async function albumAssets(token: string, albumId: number): Promise<AlbumAssets> {
  return api<AlbumAssets>(`/api/v1/albums/${albumId}/assets`, { token });
}

async function uploadOwnedAsset(workspace: Workspace, filename: string): Promise<void> {
  const source = readFileSync(workspace.uploadSource);
  const endOfImage = source.lastIndexOf(Buffer.from([0xff, 0xd9]));
  if (endOfImage < 0) throw new Error("agent runtime fixture is not a JPEG");
  const marker = Buffer.from(`lumilio-agent-runtime:${filename}`, "utf8");
  const markerLength = marker.length + 2;
  const uniqueSource = Buffer.concat([
    source.subarray(0, endOfImage),
    Buffer.from([0xff, 0xfe, markerLength >> 8, markerLength & 0xff]),
    marker,
    source.subarray(endOfImage),
  ]);
  const form = new FormData();
  form.append("repository_id", workspace.repositoryId);
  form.append("file", new Blob([uniqueSource], { type: "image/jpeg" }), filename);
  const response = await fetch(`${baseURL}/api/v1/assets`, {
    method: "POST",
    headers: { authorization: `Bearer ${workspace.token}` },
    body: form,
  });
  const body: unknown = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(`POST /api/v1/assets: ${response.status} ${JSON.stringify(body)}`);
  }
}

async function setOCREnabled(token: string, enabled: boolean): Promise<void> {
  await api("/api/v1/settings/system", {
    method: "PATCH",
    token,
    body: JSON.stringify({ ml: { ocr_enabled: enabled } }),
  });
}

async function poll(check: () => Promise<boolean>, timeout = 60_000): Promise<void> {
  const deadline = Date.now() + timeout;
  let lastError: unknown;
  while (Date.now() < deadline) {
    try {
      if (await check()) return;
    } catch (error) {
      lastError = error;
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  if (lastError) throw lastError;
  throw new Error(`agent runtime fixture did not become ready within ${timeout}ms`);
}
