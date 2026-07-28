import { readFileSync } from "node:fs";
import process from "node:process";
import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { api, baseURL } from "../support/api";
import type { components } from "../../src/lib/http-commons/schema.d.ts";
import {
  profileAsset,
  VIDEO_REGRESSION_ASSETS,
  VIDEO_REGRESSION_PHOTO_ASSET,
  VIDEO_REGRESSION_PROFILE,
} from "../support/assets";

type Asset = {
  asset_id: string;
  original_filename: string;
  type: "PHOTO" | "VIDEO";
};

// Derived from the generated OpenAPI types rather than hand-written, so a
// browse-contract change breaks the build instead of failing at runtime.
type QueryResponse = components["schemas"]["dto.QueryAssetsResponseDTO"];
type SearchResponse = components["schemas"]["dto.SearchAssetsResponseDTO"];
type BrowseItem = components["schemas"]["dto.BrowseItemDTO"];

/** Browse rows are logical media items; the file lives on the primary asset. */
function primaryAssets(items: BrowseItem[]): Asset[] {
  return items
    .map((item) => item.media_item?.primary_asset)
    .filter((asset): asset is Asset => Boolean(asset?.asset_id));
}

type Repository = {
  id: string;
  name: string;
};

type IndexingTaskStats = {
  indexed_count: number;
  queued_jobs: number;
  total_count: number;
};

type IndexingStats = {
  photo_total: number;
  video_total: number;
  reindex_jobs: number;
  tasks: {
    semantic: IndexingTaskStats;
    video_semantic: IndexingTaskStats;
  };
};

type LumenMetrics = {
  semantic_image: number;
  semantic_text: number;
};

type QueueSummary = {
  queues: {
    name: string;
    remaining_jobs: number;
  }[];
};

type MLSettings = {
  semantic_enabled: boolean;
  bioclip_enabled: boolean;
  ocr_enabled: boolean;
  face_enabled: boolean;
  video_semantic_enabled: boolean;
  video_max_frames: number;
  video_long_threshold_seconds: number;
  video_scene_threshold: number;
};

type SystemSettings = {
  ml: MLSettings;
};

const lumenMetricsURL =
  process.env.LUMILIO_E2E_LUMEN_METRICS_URL ?? "http://127.0.0.1:16658/metrics";
const frameCap = 2;

async function getLumenMetrics(): Promise<LumenMetrics> {
  const response = await fetch(lumenMetricsURL);
  if (!response.ok) throw new Error(`GET ${lumenMetricsURL}: ${response.status}`);
  return response.json() as Promise<LumenMetrics>;
}

async function ensureRepository(token: string, name: string): Promise<Repository> {
  const { repositories } = await api<{ repositories: Repository[] }>("/api/v1/repositories", {
    token,
  });
  const existing = repositories.find((repository) => repository.name === name);
  if (existing) return existing;

  const { repository } = await api<{ repository: Repository }>("/api/v1/repositories", {
    method: "POST",
    token,
    body: JSON.stringify({
      name,
      role: "regular",
      storage_strategy: "flat",
      duplicate_handling: "rename",
    }),
  });
  return repository;
}

async function uploadAsset(
  token: string,
  repositoryID: string,
  source: string,
  filename: string,
  mimeType: string,
) {
  const form = new FormData();
  form.append("repository_id", repositoryID);
  form.append(
    "file",
    new Blob([new Uint8Array(readFileSync(source))], { type: mimeType }),
    filename,
  );
  const response = await fetch(`${baseURL}/api/v1/assets`, {
    method: "POST",
    headers: { authorization: `Bearer ${token}` },
    body: form,
  });
  const body: unknown = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(`POST /api/v1/assets: ${response.status} ${JSON.stringify(body)}`);
  }
}

async function findAsset(
  token: string,
  repositoryID: string,
  filename: string,
): Promise<Asset | undefined> {
  const response = await api<QueryResponse>("/api/v1/assets/list", {
    method: "POST",
    token,
    body: JSON.stringify({
      query: filename,
      search_type: "filename",
      filter: { repository_id: repositoryID },
      pagination: { limit: 50, offset: 0 },
      stack_mode: "expanded",
    }),
  });
  return primaryAssets(response.items ?? []).find((asset) => asset.original_filename === filename);
}

async function waitForAsset(token: string, repositoryID: string, filename: string): Promise<Asset> {
  let matched: Asset | undefined;
  await expect
    .poll(
      async () => {
        matched = await findAsset(token, repositoryID, filename);
        return matched?.asset_id;
      },
      {
        message: `${filename} should finish ingest and appear in browse`,
        timeout: 120_000,
        intervals: [500, 1_000, 2_000],
      },
    )
    .toBeTruthy();
  return matched as Asset;
}

function getIndexingStats(token: string, repositoryID: string) {
  return api<IndexingStats>(
    `/api/v1/assets/indexing/stats?repository_id=${encodeURIComponent(repositoryID)}`,
    { token },
  );
}

async function waitForIndexingCoverage(
  token: string,
  repositoryID: string,
  expectedPhotos: number,
  expectedVideos: number,
) {
  await expect
    .poll(
      async () => {
        const stats = await getIndexingStats(token, repositoryID);
        return {
          photoIndexed: stats.tasks.semantic.indexed_count,
          photoTotal: stats.photo_total,
          videoIndexed: stats.tasks.video_semantic.indexed_count,
          videoTotal: stats.video_total,
        };
      },
      {
        message: "semantic indexing coverage should reach the repository totals",
        timeout: 180_000,
        intervals: [500, 1_000, 2_000],
      },
    )
    .toEqual({
      photoIndexed: expectedPhotos,
      photoTotal: expectedPhotos,
      videoIndexed: expectedVideos,
      videoTotal: expectedVideos,
    });
}

async function waitForDisabledVideoCoverage(token: string, repositoryID: string) {
  await expect
    .poll(
      async () => {
        const stats = await getIndexingStats(token, repositoryID);
        return {
          indexed: stats.tasks.video_semantic.indexed_count,
          total: stats.video_total,
        };
      },
      {
        message: "the AI-disabled video should stay intentionally unindexed",
        timeout: 120_000,
        intervals: [500, 1_000, 2_000],
      },
    )
    .toEqual({ indexed: 0, total: 1 });
}

async function waitForQueuesIdle(token: string, names: string[]) {
  await expect
    .poll(
      async () => {
        const summary = await api<QueueSummary>("/api/v1/admin/river/queue-summary", {
          token,
        });
        return Object.fromEntries(
          names.map((name) => [
            name,
            summary.queues.find((queue) => queue.name === name)?.remaining_jobs ?? 0,
          ]),
        );
      },
      {
        message: `${names.join(", ")} should become idle`,
        timeout: 180_000,
        intervals: [500, 1_000, 2_000],
      },
    )
    .toEqual(Object.fromEntries(names.map((name) => [name, 0])));
}

async function waitForImageInferencesAtLeast(previous: number, expectedDelta: number) {
  await expect
    .poll(async () => (await getLumenMetrics()).semantic_image - previous, {
      message: `the Lumen fixture should receive at least ${expectedDelta} new image inferences`,
      timeout: 180_000,
      intervals: [500, 1_000, 2_000],
    })
    .toBeGreaterThanOrEqual(expectedDelta);
}

async function waitForWebVideo(token: string, assetID: string) {
  await expect
    .poll(
      async () => {
        const response = await fetch(`${baseURL}/api/v1/assets/${assetID}/video/web`, {
          method: "HEAD",
          headers: { authorization: `Bearer ${token}` },
        });
        return response.status;
      },
      {
        message: "the AI-disabled video should still produce a playable web rendition",
        timeout: 120_000,
        intervals: [500, 1_000, 2_000],
      },
    )
    .toBe(200);
}

test("@video-regression pinned videos cover semantic indexing lifecycle", async ({
  page,
  workspace,
}, testInfo) => {
  test.setTimeout(600_000);

  const originalSettings = await api<SystemSettings>("/api/v1/settings/system", {
    token: workspace.token,
  });
  const runLabel = `Run ${testInfo.retry}`;
  const repository = await ensureRepository(
    workspace.token,
    `Video Semantic Regression ${runLabel}`,
  );
  const disabledRepository = await ensureRepository(
    workspace.token,
    `Video Semantic Disabled Regression ${runLabel}`,
  );

  try {
    await api<SystemSettings>("/api/v1/settings/system", {
      method: "PATCH",
      token: workspace.token,
      body: JSON.stringify({
        ml: {
          semantic_enabled: true,
          video_semantic_enabled: true,
          video_max_frames: frameCap,
        },
      }),
    });
    await waitForQueuesIdle(workspace.token, [
      "process_semantic",
      "process_video_frames",
      "reindex_assets",
      "retry_asset",
    ]);

    const photoFilename = "video-regression-photo.jpg";
    const photoStartedAt = Date.now();
    const beforePhoto = await getLumenMetrics();
    await uploadAsset(
      workspace.token,
      repository.id,
      profileAsset(VIDEO_REGRESSION_PROFILE, VIDEO_REGRESSION_PHOTO_ASSET),
      photoFilename,
      "image/jpeg",
    );
    await waitForAsset(workspace.token, repository.id, photoFilename);
    await waitForIndexingCoverage(workspace.token, repository.id, 1, 0);
    await waitForQueuesIdle(workspace.token, ["process_semantic"]);
    await waitForImageInferencesAtLeast(beforePhoto.semantic_image, 1);
    const photoIndexedMs = Date.now() - photoStartedAt;

    const timings: { filename: string; indexed_ms: number; frames: number }[] = [];
    const videos: Asset[] = [];
    for (const [index, fixtureID] of VIDEO_REGRESSION_ASSETS.entries()) {
      const filename = `video-regression-${index + 1}.mp4`;
      const startedAt = Date.now();
      const before = await getLumenMetrics();
      await uploadAsset(
        workspace.token,
        repository.id,
        profileAsset(VIDEO_REGRESSION_PROFILE, fixtureID),
        filename,
        "video/mp4",
      );
      const asset = await waitForAsset(workspace.token, repository.id, filename);
      videos.push(asset);
      await waitForIndexingCoverage(workspace.token, repository.id, 1, index + 1);
      await waitForQueuesIdle(workspace.token, ["process_video_frames"]);

      const after = await getLumenMetrics();
      const frames = after.semantic_image - before.semantic_image;
      expect(frames, `${filename} should produce at least one semantic frame`).toBeGreaterThan(0);
      expect(frames, `${filename} should respect video_max_frames`).toBeLessThanOrEqual(frameCap);
      timings.push({ filename, indexed_ms: Date.now() - startedAt, frames });
    }

    await test.step("photo and videos coexist in semantic text search", async () => {
      const response = await api<SearchResponse>("/api/v1/assets/search", {
        method: "POST",
        token: workspace.token,
        body: JSON.stringify({
          query: "deterministic mixed media",
          filter: { repository_id: repository.id },
          pagination: { limit: 50, offset: 0 },
          enhancement_mode: "auto",
          top_results_limit: 50,
          // No stack_mode: search results are always flat by media item, and
          // the endpoint rejects the field outright.
        }),
      });
      const assets = primaryAssets([
        ...(response.top_items ?? []),
        ...(response.result_items ?? []),
      ]);
      expect(
        assets.some((asset) => asset.original_filename === photoFilename && asset.type === "PHOTO"),
      ).toBe(true);
      expect(
        assets.some(
          (asset) =>
            asset.original_filename.startsWith("video-regression-") && asset.type === "VIDEO",
        ),
      ).toBe(true);
    });

    const initialFrameTotal = timings.reduce((sum, timing) => sum + timing.frames, 0);

    await test.step("video semantic backfill re-embeds all pinned videos", async () => {
      const before = await getLumenMetrics();
      const response = await api<{ status: string; requested_tasks: string[] }>(
        "/api/v1/assets/indexing/rebuild",
        {
          method: "POST",
          token: workspace.token,
          body: JSON.stringify({
            repository_id: repository.id,
            tasks: ["video_semantic"],
            limit: 50,
            missing_only: false,
          }),
        },
      );
      expect(response.status).toBe("queued");
      expect(response.requested_tasks).toContain("video_semantic");
      await waitForImageInferencesAtLeast(before.semantic_image, initialFrameTotal);
      await waitForQueuesIdle(workspace.token, ["reindex_assets", "process_video_frames"]);
      await waitForIndexingCoverage(workspace.token, repository.id, 1, videos.length);
    });

    await test.step("semantic reset refills photo and video indexes", async () => {
      const before = await getLumenMetrics();
      const response = await api<{ status: string; requested_tasks: string[] }>(
        "/api/v1/assets/indexing/rebuild",
        {
          method: "POST",
          token: workspace.token,
          body: JSON.stringify({
            tasks: ["semantic"],
            limit: 50,
            missing_only: false,
            reset_semantic: true,
          }),
        },
      );
      expect(response.status).toBe("queued");
      expect(response.requested_tasks).toContain("semantic");
      await waitForImageInferencesAtLeast(before.semantic_image, initialFrameTotal + 1);
      await waitForQueuesIdle(workspace.token, [
        "reindex_assets",
        "process_semantic",
        "process_video_frames",
      ]);
      await waitForIndexingCoverage(workspace.token, repository.id, 1, videos.length);
    });

    await test.step("selective retry re-runs video frame indexing", async () => {
      const before = await getLumenMetrics();
      const response = await api<{ status: string; retry_tasks: string[] }>(
        `/api/v1/assets/${videos[0].asset_id}/reprocess`,
        {
          method: "POST",
          token: workspace.token,
          body: JSON.stringify({ tasks: ["process_video_frames"] }),
        },
      );
      expect(response.status).toBe("queued");
      expect(response.retry_tasks).toEqual(["process_video_frames"]);
      await waitForImageInferencesAtLeast(before.semantic_image, timings[0].frames);
      await waitForQueuesIdle(workspace.token, ["retry_asset", "process_video_frames"]);
    });

    await test.step("disabled ML preserves video ingest, browse, and playback", async () => {
      await api<SystemSettings>("/api/v1/settings/system", {
        method: "PATCH",
        token: workspace.token,
        body: JSON.stringify({
          ml: {
            semantic_enabled: false,
            bioclip_enabled: false,
            ocr_enabled: false,
            face_enabled: false,
            video_semantic_enabled: false,
          },
        }),
      });

      const before = await getLumenMetrics();
      const filename = "video-regression-ai-disabled.mp4";
      await uploadAsset(
        workspace.token,
        disabledRepository.id,
        profileAsset(VIDEO_REGRESSION_PROFILE, VIDEO_REGRESSION_ASSETS[0]),
        filename,
        "video/mp4",
      );
      const asset = await waitForAsset(workspace.token, disabledRepository.id, filename);
      expect(asset.type).toBe("VIDEO");
      await waitForWebVideo(workspace.token, asset.asset_id);
      await waitForDisabledVideoCoverage(workspace.token, disabledRepository.id);
      expect((await getLumenMetrics()).semantic_image).toBe(before.semantic_image);

      await new LoginPage(page).signIn(workspace.username, workspace.password);
      await page.goto(`/assets/${asset.asset_id}`);
      const video = page.getByRole("region", { name: new RegExp(filename, "i") }).locator("video");
      await expect(video).toBeVisible({ timeout: 30_000 });
      await expect
        .poll(() => video.evaluate((element) => (element as HTMLVideoElement).readyState))
        .toBeGreaterThanOrEqual(1);
    });

    console.log(
      `LUMILIO_VIDEO_SEMANTIC_BASELINE ${JSON.stringify({
        kind: "cpu_ffmpeg_pipeline_with_deterministic_lumen",
        platform: process.platform,
        arch: process.arch,
        photo_indexed_ms: photoIndexedMs,
        videos: timings,
      })}`,
    );
  } finally {
    await api<SystemSettings>("/api/v1/settings/system", {
      method: "PATCH",
      token: workspace.token,
      body: JSON.stringify({ ml: originalSettings.ml }),
    });
  }
});
