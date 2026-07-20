import type { Connect, Plugin } from "vite";

// In-memory stand-in for the Wails /__onb API (desktop/onboarding.go +
// control_panel.go), used by `vp dev` when LUMILIO_PANEL_API is not set.
// It mimics shapes and timing, not real behavior.

interface MockState {
  mode: "onboarding" | "dashboard";
  region: string;
  path: string;
  ready: boolean;
  lumen: {
    enabled: boolean;
    state: string;
    error: string;
    preset: string;
    backend: string;
    profile: string;
    cacheDir: string;
    previousCacheDir: string;
    installedVersion: string;
    latestVersion: string;
    phase: string;
    download: {
      model: string;
      file: string;
      bytesDone: number;
      bytesTotal: number;
      filesDone: number;
      filesTotal: number;
    } | null;
  };
}

const HOME = "/Users/demo";

const mock: MockState = {
  mode: "onboarding",
  region: "other",
  path: `${HOME}/Pictures/Lumilio Library`,
  ready: false,
  lumen: {
    enabled: false,
    state: "",
    error: "",
    preset: "",
    backend: "",
    profile: "",
    cacheDir: `${HOME}/Library/Application Support/Lumilio Photos/lumen/models`,
    previousCacheDir: "",
    installedVersion: "",
    latestVersion: "",
    phase: "",
    download: null,
  },
};

// Simulates the hub's startup: download progress ticks, then loading, then
// ready — mirroring what the real control plane reports.
function simulateHubStartup() {
  mock.lumen.state = "starting";
  mock.lumen.phase = "downloading";
  const total = 1_200_000_000;
  mock.lumen.download = {
    model: "bioclip-v2",
    file: "burn/vision.fp32.bpk",
    bytesDone: 0,
    bytesTotal: total,
    filesDone: 1,
    filesTotal: 4,
  };
  const tick = setInterval(() => {
    const download = mock.lumen.download;
    if (!download) return clearInterval(tick);
    download.bytesDone = Math.min(download.bytesDone + total / 20, total);
    if (download.bytesDone >= total) {
      clearInterval(tick);
      mock.lumen.phase = "loading";
      mock.lumen.download = null;
      setTimeout(() => {
        mock.lumen.phase = "ready";
        mock.lumen.state = "running";
      }, 2500);
    }
  }, 500);
}

const validation = {
  reachable: true,
  writable: true,
  freeBytes: 229_000_000_000,
  freeHuman: "213.4 GB",
};

const LOGS: Record<string, string> = {
  app: "2026-07-17 09:12:03 INFO  server listening on 127.0.0.1:6680\n2026-07-17 09:12:04 INFO  library scan complete: 4,213 items\n2026-07-17 09:14:41 INFO  thumbnail cache warmed (812MB)",
  error: "2026-07-16 22:03:11 WARN  slow query: 1.4s (media_search)",
  postgres:
    "2026-07-17 09:11:58 INFO  postgres ready, 213ms startup\n2026-07-17 09:12:00 INFO  migrations up to date (v42)",
  lumen:
    "2026-07-17 09:13:20 INFO  lumen-hub booting, backend=metal\n2026-07-17 09:13:26 INFO  model loaded: bioclip-v2 (1.2GB)",
};

function statePayload() {
  return {
    ...mock,
    lang: "en",
    validation,
    version: "0.9.0-dev",
    tosRev: "dev",
    serverURL: "http://localhost:6680",
    stage: mock.ready ? "running" : "starting",
    paths: {
      storage: mock.path,
      logs: `${HOME}/Library/Logs/Lumilio Photos`,
      backups: `${HOME}/Library/Application Support/Lumilio Photos/backups`,
      appData: `${HOME}/Library/Application Support/Lumilio Photos`,
    },
    backends: [
      { name: "metal", profile: "darwin-arm64-metal", recommended: true },
      { name: "cpu", profile: "darwin-arm64-cpu" },
    ],
    presets: [
      { name: "minimal", minRamGB: 4, minDiskGB: 2 },
      { name: "basic", minRamGB: 6, minDiskGB: 6 },
      { name: "brave", minRamGB: 8, minDiskGB: 10 },
    ],
    recommendedPreset: "basic",
    memoryGB: 16,
    cacheValidation: validation,
  };
}

function json(res: Parameters<Connect.NextHandleFunction>[1], body: unknown) {
  res.setHeader("Content-Type", "application/json");
  res.end(JSON.stringify(body));
}

async function readBody(req: Parameters<Connect.NextHandleFunction>[0]): Promise<any> {
  const chunks: Buffer[] = [];
  for await (const chunk of req) chunks.push(chunk as Buffer);
  const raw = Buffer.concat(chunks).toString("utf8");
  return raw ? JSON.parse(raw) : {};
}

export function mockPanelApi(): Plugin {
  return {
    name: "lumilio-mock-panel-api",
    apply: "serve",
    configureServer(server) {
      server.middlewares.use(async (req, res, next) => {
        const url = new URL(req.url ?? "/", "http://localhost");
        if (!url.pathname.startsWith("/__onb/")) return next();

        switch (url.pathname) {
          case "/__onb/state":
            return json(res, statePayload());
          case "/__onb/pick":
            return json(res, { path: `${HOME}/Pictures/Lumilio Library`, validation });
          case "/__onb/pick-cache":
            return json(res, { path: `${HOME}/Library/Caches/lumen-models`, validation });
          case "/__onb/complete": {
            const body = await readBody(req);
            mock.mode = "dashboard";
            mock.path = body.path ?? mock.path;
            mock.region = body.region ?? mock.region;
            mock.lumen.enabled = Boolean(body.enableLumen);
            if (body.enableLumen) {
              Object.assign(mock.lumen, {
                preset: body.preset,
                backend: body.backend,
                profile: body.profile,
                cacheDir: body.cacheDir,
                state: "installing",
                installedVersion: "0.9.1",
                latestVersion: "0.10.0",
              });
              setTimeout(simulateHubStartup, 1500);
            }
            setTimeout(() => (mock.ready = true), 4000);
            return json(res, { ok: true });
          }
          case "/__onb/region": {
            const body = await readBody(req);
            mock.region = body.region === "cn" ? "cn" : "other";
            return json(res, { ok: true, region: mock.region });
          }
          case "/__onb/lumen-save": {
            const body = await readBody(req);
            if (mock.lumen.cacheDir !== body.cacheDir) {
              mock.lumen.previousCacheDir = mock.lumen.cacheDir;
            }
            Object.assign(mock.lumen, body);
            return json(res, { ok: true });
          }
          case "/__onb/lumen-action": {
            const body = await readBody(req);
            const a = body.action as string;
            if (a === "enable") {
              mock.lumen.enabled = true;
              simulateHubStartup();
            } else if (a === "disable") {
              mock.lumen.enabled = false;
              mock.lumen.state = "off";
              mock.lumen.phase = "";
              mock.lumen.download = null;
            } else if (a === "restart") {
              simulateHubStartup();
            } else if (a === "check") {
              mock.lumen.latestVersion = "0.10.0";
            } else if (a === "update") {
              mock.lumen.state = "installing";
              setTimeout(() => {
                mock.lumen.installedVersion = mock.lumen.latestVersion;
                mock.lumen.state = "running";
              }, 4000);
            }
            return json(res, { ok: true });
          }
          case "/__onb/log": {
            const source = url.searchParams.get("source") ?? "app";
            return json(res, {
              content: LOGS[source] ?? "",
              path: `${HOME}/Library/Logs/Lumilio Photos/${source}.log`,
            });
          }
          case "/__onb/open":
          case "/__onb/open-app":
            return json(res, { ok: true });
          case "/__onb/legal/license":
            res.end("GNU GENERAL PUBLIC LICENSE\nVersion 3 (mock)\n\n…");
            return;
          case "/__onb/legal/third-party":
            res.end("THIRD-PARTY NOTICES (mock)\n\npostgres — PostgreSQL License\n…");
            return;
          case "/__onb/legal/terms":
            res.end("TERMS OF USE (mock)\n\nLumilio Photos runs entirely on your machine.\n…");
            return;
          default:
            res.statusCode = 404;
            res.end("mock: not found");
            return;
        }
      });
    },
  };
}
