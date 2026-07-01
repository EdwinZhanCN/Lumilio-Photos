import type { components, paths } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];
type Paths = paths;

export type DuplicateSummary = Schemas["dto.DuplicateSummaryDTO"];
export type DuplicateGroup = Schemas["dto.DuplicateGroupDTO"];
export type DuplicateAsset = Schemas["dto.DuplicateAssetDTO"];
export type DuplicateEdge = Schemas["dto.DuplicateEdgeDTO"];
export type ListDuplicateGroupsResponse = Schemas["dto.ListDuplicateGroupsResponseDTO"];
export type DetectDuplicatesRequest = Schemas["dto.DetectDuplicatesRequestDTO"];
export type DetectDuplicatesResponse = Schemas["dto.DetectDuplicatesResponseDTO"];
export type MergeDuplicateGroupRequest = Schemas["dto.MergeDuplicateGroupRequestDTO"];
export type MergeDuplicateGroupResponse = Schemas["dto.MergeDuplicateGroupResponseDTO"];
export type MergeDuplicatePolicy = Schemas["dto.MergeDuplicatePolicyDTO"];

export type ListDuplicateGroupsParams = NonNullable<
  Paths["/api/v1/duplicates/groups"]["get"]["parameters"]["query"]
>;

export type DuplicateStatus = "pending" | "merged" | "dismissed";
export type DuplicateMethod = "exact" | "phash" | "mixed";
