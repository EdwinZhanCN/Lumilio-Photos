// src/services/queueService.ts

import api from "@/lib/http-commons/api.ts";
import type { AxiosResponse } from "axios";

// ============================================================================
// Type Definitions
// ============================================================================

export interface JobDTO {
  id: number;
  queue: string;
  kind: string;
  state: JobState;
  attempt: number;
  max_attempts: number;
  priority: number;
  scheduled_at: string;
  created_at: string;
  attempted_at?: string;
  finalized_at?: string;
  errors?: string[];
  args?: any;
  metadata?: any;
}

export type JobState =
  | "available"
  | "scheduled"
  | "running"
  | "retryable"
  | "completed"
  | "cancelled"
  | "discarded";

export interface JobListResponse {
  jobs: JobDTO[];
  cursor?: string;
  total_count?: number;
}

export interface QueueStatsDTO {
  name: string;
  updated_at: string;
  metadata?: any;
}

export interface QueueStatsResponse {
  queues: QueueStatsDTO[];
}

export interface JobStatsResponse {
  available: number;
  scheduled: number;
  running: number;
  retryable: number;
  completed: number;
  cancelled: number;
  discarded: number;
}

export interface JobListParams {
  state?: JobState;
  queue?: string;
  kind?: string;
  limit?: number;
  cursor?: string;
  time_range?: "1h" | "24h" | "30d";
  include_count?: boolean;
}

// ============================================================================
// API Result Wrapper
// ============================================================================

export interface ApiResult<T = any> {
  code: number;
  message: string;
  data?: T;
}

// ============================================================================
// Queue Service Functions
// ============================================================================

/**
 * List jobs with optional filters
 */
export async function listJobs(
  params?: JobListParams,
): Promise<JobListResponse> {
  const queryParams = new URLSearchParams();

  if (params?.state) queryParams.append("state", params.state);
  if (params?.queue) queryParams.append("queue", params.queue);
  if (params?.kind) queryParams.append("kind", params.kind);
  if (params?.limit) queryParams.append("limit", params.limit.toString());
  if (params?.cursor) queryParams.append("cursor", params.cursor);
  if (params?.time_range) queryParams.append("time_range", params.time_range);
  if (params?.include_count) queryParams.append("include_count", "true");

  const url = `/api/v1/admin/river/jobs${queryParams.toString() ? `?${queryParams.toString()}` : ""}`;

  const response: AxiosResponse<ApiResult<JobListResponse>> =
    await api.get(url);
  return response.data.data!;
}

/**
 * Get a single job by ID
 */
export async function getJob(jobId: number): Promise<JobDTO> {
  const response: AxiosResponse<ApiResult<JobDTO>> = await api.get(
    `/api/v1/admin/river/jobs/${jobId}`,
  );
  return response.data.data!;
}

/**
 * List all active queues
 */
export async function listQueues(limit?: number): Promise<QueueStatsResponse> {
  const url = `/api/v1/admin/river/queues${limit ? `?limit=${limit}` : ""}`;
  const response: AxiosResponse<ApiResult<QueueStatsResponse>> =
    await api.get(url);
  return response.data.data!;
}

/**
 * Get aggregated job statistics by state
 */
export async function getJobStats(): Promise<JobStatsResponse> {
  const response: AxiosResponse<ApiResult<JobStatsResponse>> = await api.get(
    `/api/v1/admin/river/stats`,
  );
  return response.data.data!;
}

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
      onUpdate(stats);
    } catch (error) {
      console.error("Failed to fetch job stats:", error);
    }
  };

  // Initial fetch
  poll();

  // Start interval
  const intervalId = setInterval(poll, intervalMs);

  // Return cleanup function
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
      onUpdate(response.jobs);
    } catch (error) {
      console.error("Failed to fetch job list:", error);
    }
  };

  // Initial fetch
  poll();

  // Start interval
  const intervalId = setInterval(poll, intervalMs);

  // Return cleanup function
  return () => clearInterval(intervalId);
}

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
