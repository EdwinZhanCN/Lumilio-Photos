import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type PersonSummary = Schemas["dto.PersonSummaryDTO"];
export type PersonDetail = Schemas["dto.PersonDetailDTO"];
export type ListPeopleResponse = Schemas["dto.ListPeopleResponseDTO"];
export type UpdatePersonRequest = Schemas["dto.UpdatePersonRequestDTO"];
