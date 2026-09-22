import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type ProcessingMonitorResponse = Schemas["handler.ProcessingMonitorResponse"];
export type QueueErrorSampleDTO = Schemas["handler.QueueErrorSampleDTO"];
export type QueueSummaryDTO = Schemas["handler.QueueSummaryDTO"];
