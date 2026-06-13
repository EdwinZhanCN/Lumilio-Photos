import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type JobStatsResponse = Schemas["handler.JobStatsResponse"];
export type QueueErrorSampleDTO = Schemas["handler.QueueErrorSampleDTO"];
export type QueueSummaryDTO = Schemas["handler.QueueSummaryDTO"];
export type QueueSummaryResponse = Schemas["handler.QueueSummaryResponse"];

export type JobState =
  | "available"
  | "scheduled"
  | "running"
  | "retryable"
  | "completed"
  | "cancelled"
  | "discarded";
