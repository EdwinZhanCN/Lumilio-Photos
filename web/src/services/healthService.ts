import api from "@/lib/http-commons/api.ts";
import type { AxiosResponse } from "axios";

export const HEALTH_ENDPOINT = "/api/v1/health";
export const MIN_HEALTH_INTERVAL_SEC = 1;
export const MAX_HEALTH_INTERVAL_SEC = 50;
export const DEFAULT_HEALTH_INTERVAL_SEC = 5;

export interface HealthCheckResult<T = any> {
  online: boolean;
  statusCode: number;
  data?: T;
  error?: string;
}

/**
  Fetch raw health response using shared axios instance.
  Consumers rarely need this directly. Prefer checkHealth or isServerOnline.
*/
export async function fetchHealth<T = any>(): Promise<AxiosResponse<T>> {
  // Accept non-2xx for explicit online=false classification without throwing
  return api.get<T>(HEALTH_ENDPOINT, { validateStatus: () => true });
}

/**
  Calls /api/v1/health and returns normalized result.
  online is true for 2xx responses, false otherwise (including network errors).
*/
export async function checkHealth<T = any>(): Promise<HealthCheckResult<T>> {
  try {
    const res = await fetchHealth<T>();
    const online = res.status >= 200 && res.status < 300;
    return {
      online,
      statusCode: res.status,
      data: res.data,
    };
  } catch (err: any) {
    return {
      online: false,
      statusCode: 0,
      error:
        typeof err?.message === "string"
          ? err.message
          : "Network or CORS error",
    };
  }
}

/**
  Lightweight convenience wrapper to get online/offline state.
*/
export async function isServerOnline(): Promise<boolean> {
  const result = await checkHealth();
  return result.online;
}

/**
  Starts polling the health endpoint at the specified interval (in seconds).
  Invokes onUpdate with the latest HealthCheckResult after each poll.
  Returns a cleanup function to stop polling.

  Example:
    const stop = pollHealth(5, ({ online }) => setOnline(online));
    // later: stop()
*/
export function pollHealth<T = any>(
  intervalSeconds: number,
  onUpdate: (result: HealthCheckResult<T>) => void,
): () => void {
  const ms = Math.max(
    1000,
    Math.min(MAX_HEALTH_INTERVAL_SEC, Math.max(MIN_HEALTH_INTERVAL_SEC, intervalSeconds)) * 1000,
  );

  let cancelled = false;
  let timer: number | undefined;

  const run = async () => {
    if (cancelled) return;
    const result = await checkHealth<T>();
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
