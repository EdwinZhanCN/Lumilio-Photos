import type { components, paths } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type JobDTO = Schemas["handler.JobDTO"];
export type JobListResponse = Schemas["handler.JobListResponse"];
export type JobStatsResponse = Schemas["handler.JobStatsResponse"];
export type QueueStatsDTO = Schemas["handler.QueueStatsDTO"];
export type QueueStatsResponse = Schemas["handler.QueueStatsResponse"];

export type JobListParams = NonNullable<
  paths["/api/v1/admin/river/jobs"]["get"]["parameters"]["query"]
>;

export type JobState =
  | "available"
  | "scheduled"
  | "running"
  | "retryable"
  | "completed"
  | "cancelled"
  | "discarded";
