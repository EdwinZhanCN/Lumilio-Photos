#!/usr/bin/env node

import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const POLL_INTERVAL_MS = parseIntegerEnv("LUMILIO_MONITOR_INTERVAL_MS", 10_000);
const IDLE_POLLS_TO_FINISH = parseIntegerEnv("LUMILIO_MONITOR_IDLE_POLLS", 3);
const BASE_URL = process.env.LUMILIO_BASE_URL ?? "http://localhost:8080";
const API_BASE = new URL("/api/v1", ensureTrailingSlash(BASE_URL)).toString().replace(/\/$/, "");
const CONTAINER_NAME = process.env.LUMILIO_CONTAINER ?? "";
const OUTPUT_DIR =
  process.env.LUMILIO_MONITOR_OUTPUT_DIR ??
  path.join(os.tmpdir(), "lumilio-import-monitor", timestampForPath(new Date()));

const TRACKED_QUEUES = ["ingest_asset", "thumbnail_asset", "process_phash"];
const ACTIVE_STATES = ["available", "scheduled", "running", "retryable"];

const authHeaders = buildAuthHeaders();
const outputFiles = prepareOutputFiles(OUTPUT_DIR);

const monitorState = {
  startedAt: new Date(),
  firstActivityAt: null,
  idlePolls: 0,
  peaks: {
    cpuPercent: 0,
    memoryBytes: 0,
    memoryPercent: 0,
  },
  queues: Object.fromEntries(
    TRACKED_QUEUES.map((queue) => [
      queue,
      {
        seenActivity: false,
        firstActiveAt: null,
        drainedAt: null,
        peakPending: 0,
      },
    ]),
  ),
};

let shouldStop = false;

process.on("SIGINT", () => {
  shouldStop = true;
  console.log("");
  console.log("Received SIGINT, finishing after the current sample...");
});

console.log(`Monitoring import activity against ${API_BASE}`);
console.log(`Writing samples to ${outputFiles.samplesPath}`);
console.log(`Writing summary to ${outputFiles.summaryPath}`);
if (CONTAINER_NAME) {
  console.log(`Sampling container stats from ${CONTAINER_NAME}`);
} else {
  console.log("No container configured; set LUMILIO_CONTAINER to capture docker stats.");
}
if (Object.keys(authHeaders).length === 0) {
  console.log("No auth headers configured; set LUMILIO_BEARER_TOKEN or LUMILIO_AUTH_COOKIE if admin APIs require auth.");
}

main().catch((error) => {
  console.error(error instanceof Error ? error.stack ?? error.message : String(error));
  process.exitCode = 1;
});

async function main() {
  while (true) {
    const sampledAt = new Date();
    const [queueStats, containerStats] = await Promise.all([
      collectQueueStats(sampledAt),
      collectContainerStats(sampledAt),
    ]);

    const sample = {
      sampled_at: sampledAt.toISOString(),
      queues: queueStats,
      container: containerStats,
    };
    fs.appendFileSync(outputFiles.samplesPath, `${JSON.stringify(sample)}\n`);

    updateMonitorState(sampledAt, queueStats, containerStats);

    printSample(queueStats, containerStats);

    if (shouldStop || shouldFinish(queueStats)) {
      break;
    }

    await sleep(POLL_INTERVAL_MS);
  }

  const summary = buildSummary();
  fs.writeFileSync(outputFiles.summaryPath, `${JSON.stringify(summary, null, 2)}\n`);

  console.log("");
  console.log("Import monitoring complete.");
  console.log(`Summary: ${outputFiles.summaryPath}`);
  console.log(`Samples: ${outputFiles.samplesPath}`);
}

function updateMonitorState(sampledAt, queueStats, containerStats) {
  const anyActive = TRACKED_QUEUES.some((queue) => queueStats[queue].pending > 0);

  if (anyActive && !monitorState.firstActivityAt) {
    monitorState.firstActivityAt = sampledAt;
  }

  for (const queue of TRACKED_QUEUES) {
    const stats = queueStats[queue];
    const state = monitorState.queues[queue];

    if (stats.pending > 0) {
      state.seenActivity = true;
      if (!state.firstActiveAt) {
        state.firstActiveAt = sampledAt;
      }
      state.peakPending = Math.max(state.peakPending, stats.pending);
      state.drainedAt = null;
    } else if (state.seenActivity && !state.drainedAt) {
      state.drainedAt = sampledAt;
    }
  }

  if (typeof containerStats.cpu_percent === "number") {
    monitorState.peaks.cpuPercent = Math.max(monitorState.peaks.cpuPercent, containerStats.cpu_percent);
  }
  if (typeof containerStats.memory_usage_bytes === "number") {
    monitorState.peaks.memoryBytes = Math.max(monitorState.peaks.memoryBytes, containerStats.memory_usage_bytes);
  }
  if (typeof containerStats.memory_percent === "number") {
    monitorState.peaks.memoryPercent = Math.max(monitorState.peaks.memoryPercent, containerStats.memory_percent);
  }
}

function shouldFinish(queueStats) {
  if (!monitorState.firstActivityAt) {
    return false;
  }

  const allIdle = TRACKED_QUEUES.every((queue) => queueStats[queue].pending === 0);
  if (!allIdle) {
    monitorState.idlePolls = 0;
    return false;
  }

  monitorState.idlePolls += 1;
  return monitorState.idlePolls >= IDLE_POLLS_TO_FINISH;
}

function buildSummary() {
  const endedAt = new Date();

  return {
    started_at: monitorState.startedAt.toISOString(),
    first_activity_at: monitorState.firstActivityAt?.toISOString() ?? null,
    ended_at: endedAt.toISOString(),
    monitor_duration_seconds: secondsBetween(monitorState.startedAt, endedAt),
    active_duration_seconds: monitorState.firstActivityAt
      ? secondsBetween(monitorState.firstActivityAt, endedAt)
      : null,
    output: outputFiles,
    peaks: {
      cpu_percent: round(monitorState.peaks.cpuPercent),
      memory_bytes: monitorState.peaks.memoryBytes || null,
      memory_human: monitorState.peaks.memoryBytes ? formatBytes(monitorState.peaks.memoryBytes) : null,
      memory_percent: round(monitorState.peaks.memoryPercent),
    },
    queues: Object.fromEntries(
      TRACKED_QUEUES.map((queue) => {
        const queueState = monitorState.queues[queue];
        return [
          queue,
          {
            seen_activity: queueState.seenActivity,
            first_active_at: queueState.firstActiveAt?.toISOString() ?? null,
            drained_at: queueState.drainedAt?.toISOString() ?? null,
            peak_pending: queueState.peakPending,
            active_seconds:
              queueState.firstActiveAt && queueState.drainedAt
                ? secondsBetween(queueState.firstActiveAt, queueState.drainedAt)
                : null,
          },
        ];
      }),
    ),
  };
}

async function collectQueueStats(sampledAt) {
  const entries = await Promise.all(
    TRACKED_QUEUES.map(async (queue) => {
      const byState = {};
      let pending = 0;

      for (const state of ACTIVE_STATES) {
        const count = await fetchQueueCount(queue, state);
        byState[state] = count;
        pending += count;
      }

      return [
        queue,
        {
          sampled_at: sampledAt.toISOString(),
          pending,
          by_state: byState,
        },
      ];
    }),
  );

  return Object.fromEntries(entries);
}

async function fetchQueueCount(queue, state) {
  const url = new URL("/api/v1/admin/river/jobs", ensureTrailingSlash(BASE_URL));
  url.searchParams.set("queue", queue);
  url.searchParams.set("state", state);
  url.searchParams.set("limit", "1");
  url.searchParams.set("include_count", "true");

  const response = await fetch(url, {
    headers: {
      Accept: "application/json",
      ...authHeaders,
    },
  });

  if (!response.ok) {
    throw new Error(`Queue stats request failed for ${queue}/${state}: ${response.status} ${response.statusText}`);
  }

  const payload = await response.json();
  return Number(payload?.data?.total_count ?? 0);
}

async function collectContainerStats(sampledAt) {
  if (!CONTAINER_NAME) {
    return {
      sampled_at: sampledAt.toISOString(),
      enabled: false,
    };
  }

  try {
    const stdout = execFileSync(
      "docker",
      ["stats", "--no-stream", "--format", "{{json .}}", CONTAINER_NAME],
      { encoding: "utf8" },
    ).trim();

    if (!stdout) {
      return { sampled_at: sampledAt.toISOString(), enabled: true, missing: true };
    }

    const parsed = JSON.parse(stdout);
    const memory = parseMemoryField(parsed.MemUsage);

    return {
      sampled_at: sampledAt.toISOString(),
      enabled: true,
      name: parsed.Name ?? CONTAINER_NAME,
      cpu_percent: parsePercent(parsed.CPUPerc),
      memory_percent: parsePercent(parsed.MemPerc),
      memory_usage_bytes: memory.usedBytes,
      memory_limit_bytes: memory.limitBytes,
      memory_usage_human: memory.usedHuman,
      memory_limit_human: memory.limitHuman,
      net_io: parsed.NetIO ?? null,
      block_io: parsed.BlockIO ?? null,
      pids: parsed.PIDs ? Number(parsed.PIDs) : null,
    };
  } catch (error) {
    return {
      sampled_at: sampledAt.toISOString(),
      enabled: true,
      error: error instanceof Error ? error.message : String(error),
    };
  }
}

function buildAuthHeaders() {
  const headers = {};

  if (process.env.LUMILIO_BEARER_TOKEN) {
    headers.Authorization = `Bearer ${process.env.LUMILIO_BEARER_TOKEN}`;
  }
  if (process.env.LUMILIO_AUTH_COOKIE) {
    headers.Cookie = process.env.LUMILIO_AUTH_COOKIE;
  }

  return headers;
}

function prepareOutputFiles(outputDir) {
  fs.mkdirSync(outputDir, { recursive: true });
  return {
    outputDir,
    samplesPath: path.join(outputDir, "samples.jsonl"),
    summaryPath: path.join(outputDir, "summary.json"),
  };
}

function printSample(queueStats, containerStats) {
  const queueSummary = TRACKED_QUEUES.map((queue) => `${queue}=${queueStats[queue].pending}`).join(" ");
  const cpuSummary =
    typeof containerStats.cpu_percent === "number" ? ` cpu=${round(containerStats.cpu_percent)}%` : "";
  const memSummary =
    typeof containerStats.memory_usage_bytes === "number"
      ? ` mem=${formatBytes(containerStats.memory_usage_bytes)}`
      : "";

  console.log(`${new Date().toISOString()} ${queueSummary}${cpuSummary}${memSummary}`);
}

function parseMemoryField(value) {
  if (typeof value !== "string" || !value.includes("/")) {
    return {
      usedBytes: null,
      limitBytes: null,
      usedHuman: null,
      limitHuman: null,
    };
  }

  const [usedRaw, limitRaw] = value.split("/").map((part) => part.trim());
  return {
    usedBytes: parseByteSize(usedRaw),
    limitBytes: parseByteSize(limitRaw),
    usedHuman: usedRaw,
    limitHuman: limitRaw,
  };
}

function parsePercent(value) {
  if (typeof value !== "string") {
    return null;
  }

  const numeric = Number.parseFloat(value.replace("%", ""));
  return Number.isFinite(numeric) ? numeric : null;
}

function parseByteSize(value) {
  if (typeof value !== "string") {
    return null;
  }

  const normalized = value.replace(/\s+/g, "");
  const match = normalized.match(/^([\d.]+)([KMGTP]?i?B)$/i);
  if (!match) {
    return null;
  }

  const amount = Number.parseFloat(match[1]);
  const unit = match[2].toUpperCase();
  const multipliers = {
    B: 1,
    KB: 1_000,
    MB: 1_000_000,
    GB: 1_000_000_000,
    TB: 1_000_000_000_000,
    KIB: 1024,
    MIB: 1024 ** 2,
    GIB: 1024 ** 3,
    TIB: 1024 ** 4,
  };

  const multiplier = multipliers[unit];
  return multiplier ? Math.round(amount * multiplier) : null;
}

function formatBytes(bytes) {
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  return `${round(value)} ${units[unitIndex]}`;
}

function secondsBetween(start, end) {
  return round((end.getTime() - start.getTime()) / 1000);
}

function round(value) {
  return Math.round(value * 100) / 100;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function parseIntegerEnv(name, fallback) {
  const raw = process.env[name];
  if (!raw) {
    return fallback;
  }

  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function ensureTrailingSlash(value) {
  return value.endsWith("/") ? value : `${value}/`;
}

function timestampForPath(date) {
  return date.toISOString().replaceAll(":", "-");
}
