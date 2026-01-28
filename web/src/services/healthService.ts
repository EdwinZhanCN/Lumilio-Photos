// src/services/healthService.ts

import client from "@/lib/http-commons/client";

// ============================================================================
// Constants
// ============================================================================

export const HEALTH_ENDPOINT = "/api/v1/health" as const;
export const MIN_HEALTH_INTERVAL_SEC = 1;
export const MAX_HEALTH_INTERVAL_SEC = 50;
export const DEFAULT_HEALTH_INTERVAL_SEC = 5;

// ============================================================================
// Health Check Types
// ============================================================================

export interface HealthCheckResult {
  online: boolean;
  statusCode: number;
  data?: { status?: string };
  error?: string;
}

// ============================================================================
// Health Service
// ============================================================================

/**
 * Fetch raw health response using openapi-fetch client.
 * Returns the typed response from the /health endpoint.
 */
export async function fetchHealth() {
  return client.GET("/api/v1/health", {});
}

/**
 * Calls /health and returns normalized result.
 * online is true for 2xx responses, false otherwise (including network errors).
 */
export async function checkHealth(): Promise<HealthCheckResult> {
  try {
    const { data, error, response } = await fetchHealth();
    const online = response.status >= 200 && response.status < 300;

    return {
      online,
      statusCode: response.status,
      data: data?.data as { status?: string } | undefined,
      error: error ? String(error) : undefined,
    };
  } catch (err: unknown) {
    return {
      online: false,
      statusCode: 0,
      error:
        err instanceof Error
          ? err.message
          : "Network or CORS error",
    };
  }
}

/**
 * Lightweight convenience wrapper to get online/offline state.
 */
export async function isServerOnline(): Promise<boolean> {
  const result = await checkHealth();
  return result.online;
}

/**
 * Starts polling the health endpoint at the specified interval (in seconds).
 * Invokes onUpdate with the latest HealthCheckResult after each poll.
 * Returns a cleanup function to stop polling.
 *
 * @example
 * const stop = pollHealth(5, ({ online }) => setOnline(online));
 * // later: stop()
 */
export function pollHealth(
  intervalSeconds: number,
  onUpdate: (result: HealthCheckResult) => void,
): () => void {
  const ms = Math.max(
    1000,
    Math.min(
      MAX_HEALTH_INTERVAL_SEC,
      Math.max(MIN_HEALTH_INTERVAL_SEC, intervalSeconds),
    ) * 1000,
  );

  let cancelled = false;
  let timer: number | undefined;

  const run = async () => {
    if (cancelled) return;
    const result = await checkHealth();
    if (!cancelled) onUpdate(result);
  };

  // Immediate check, then interval
  void run();
  timer = window.setInterval(run, ms);

  return () => {
    cancelled = true;
    if (timer !== undefined) {
      window.clearInterval(timer);
    }
  };
}
