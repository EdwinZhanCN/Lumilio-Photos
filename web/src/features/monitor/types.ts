import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type DeliveryStatsDTO = Schemas["handler.DeliveryStatsDTO"];
export type QueueErrorSampleDTO = Schemas["handler.QueueErrorSampleDTO"];
export type QueueSummaryDTO = Schemas["handler.QueueSummaryDTO"];
