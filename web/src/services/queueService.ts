// src/services/queueService.ts

import client from "@/lib/http-commons/client";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

// ============================================================================
// Type Definitions from Schema
// ============================================================================

type Schemas = components["schemas"];

export type JobDTO = Schemas["handler.JobDTO"];
export type JobListResponse = Schemas["handler.JobListResponse"];
export type QueueStatsDTO = Schemas["handler.QueueStatsDTO"];
export type QueueStatsResponse = Schemas["handler.QueueStatsResponse"];
export type JobStatsResponse = Schemas["handler.JobStatsResponse"];

export type JobState =
  | "available"
  | "scheduled"
  | "running"
  | "retryable"
  | "completed"
  | "cancelled"
  | "discarded";

export interface JobListParams {
  state?: JobState;
  queue?: string;
  kind?: string;
  limit?: number;
  cursor?: string;
  time_range?: "1h" | "24h" | "30d";
  include_count?: boolean;
}

// Legacy API Result wrapper type for backwards compatibility
export interface ApiResult<T = unknown> {
  code: number;
  message: string;
  data?: T;
}

// ============================================================================
// Queue Service (Direct API calls)
// ============================================================================

/**
 * List jobs with optional filters
 */
export async function listJobs(params?: JobListParams) {
  const { data } = await client.GET("/api/v1/admin/river/jobs", {
    params: { query: params },
  });
  return data?.data as JobListResponse | undefined;
}

/**
 * Get a single job by ID
 */
export async function getJob(jobId: number) {
  const { data } = await client.GET("/api/v1/admin/river/jobs/{id}", {
    params: { path: { id: jobId } },
  });
  return data?.data as JobDTO | undefined;
}

/**
 * List all active queues
 */
export async function listQueues(limit?: number) {
  const { data } = await client.GET("/api/v1/admin/river/queues", {
    params: { query: { limit } },
  });
  return data?.data as QueueStatsResponse | undefined;
}

/**
 * Get aggregated job statistics by state
 */
export async function getJobStats() {
  const { data } = await client.GET("/api/v1/admin/river/stats", {});
  return data?.data as JobStatsResponse | undefined;
}

// ============================================================================
// React Query Hooks
// ============================================================================

/**
 * Hook for listing jobs
 */
export const useJobs = (params?: JobListParams) =>
  $api.useQuery("get", "/api/v1/admin/river/jobs", {
    params: { query: params },
  });

/**
 * Hook for getting a single job
 */
export const useJob = (jobId: number) =>
  $api.useQuery("get", "/api/v1/admin/river/jobs/{id}", {
    params: { path: { id: jobId } },
  });

/**
 * Hook for listing queues
 */
export const useQueues = (limit?: number) =>
  $api.useQuery("get", "/api/v1/admin/river/queues", {
    params: { query: { limit } },
  });

/**
 * Hook for job stats
 */
export const useJobStats = () =>
  $api.useQuery("get", "/api/v1/admin/river/stats", {});

// ============================================================================
// Polling Functions
// ============================================================================

/**
 * Start polling job stats at specified interval (in seconds)
 * Returns a cleanup function to stop polling
 */
export function pollJobStats(
  intervalSec: number,
  onUpdate: (stats: JobStatsResponse) => void,
): () => void {
  const intervalMs = Math.max(1000, intervalSec * 1000);

  const poll = async () => {
    try {
      const stats = await getJobStats();
      if (stats) onUpdate(stats);
    } catch (error) {
      console.error("Failed to fetch job stats:", error);
    }
  };

  poll();
  const intervalId = setInterval(poll, intervalMs);
  return () => clearInterval(intervalId);
}

/**
 * Start polling job list at specified interval (in seconds)
 * Returns a cleanup function to stop polling
 */
export function pollJobList(
  params: JobListParams | undefined,
  intervalSec: number,
  onUpdate: (jobs: JobDTO[]) => void,
): () => void {
  const intervalMs = Math.max(1000, intervalSec * 1000);

  const poll = async () => {
    try {
      const response = await listJobs(params);
      if (response?.jobs) onUpdate(response.jobs);
    } catch (error) {
      console.error("Failed to fetch job list:", error);
    }
  };

  poll();
  const intervalId = setInterval(poll, intervalMs);
  return () => clearInterval(intervalId);
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Get human-readable duration from timestamps
 */
export function getJobDuration(job: JobDTO): string | null {
  if (!job.attempted_at) return null;

  const start = new Date(job.attempted_at);
  const end = job.finalized_at ? new Date(job.finalized_at) : new Date();

  const durationMs = end.getTime() - start.getTime();

  if (durationMs < 1000) return `${durationMs}ms`;
  if (durationMs < 60000) return `${(durationMs / 1000).toFixed(1)}s`;
  if (durationMs < 3600000) return `${(durationMs / 60000).toFixed(1)}m`;
  return `${(durationMs / 3600000).toFixed(1)}h`;
}

/**
 * Get state display color (for UI badges)
 */
export function getStateColor(state: JobState): string {
  switch (state) {
    case "completed":
      return "success";
    case "running":
      return "info";
    case "available":
    case "scheduled":
      return "neutral";
    case "retryable":
      return "warning";
    case "cancelled":
    case "discarded":
      return "error";
    default:
      return "ghost";
  }
}

/**
 * Get state display icon
 */
export function getStateIcon(state: JobState): string {
  switch (state) {
    case "completed":
      return "✓";
    case "running":
      return "⟳";
    case "available":
      return "○";
    case "scheduled":
      return "⏱";
    case "retryable":
      return "↻";
    case "cancelled":
      return "✕";
    case "discarded":
      return "⚠";
    default:
      return "•";
  }
}
