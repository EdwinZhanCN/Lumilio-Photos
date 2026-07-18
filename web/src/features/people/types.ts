import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

export type PersonSummary = Schemas["dto.PersonSummaryDTO"];
export type PersonDetail = Schemas["dto.PersonDetailDTO"];
export type ListPeopleResponse = Schemas["dto.ListPeopleResponseDTO"];
export type FaceClusterRebuildResponse = Schemas["dto.FaceClusterRebuildResponseDTO"];
export type PersonSummaryList = NonNullable<ListPeopleResponse["people"]>;
export type UpdatePersonRequest = Schemas["dto.UpdatePersonRequestDTO"];

export type PersonFace = Schemas["dto.PersonFaceDTO"];
export type ListPersonFacesResponse = Schemas["dto.ListPersonFacesResponseDTO"];
export type PersonFaceList = NonNullable<ListPersonFacesResponse["faces"]>;
export type MergePeopleRequest = Schemas["dto.MergePeopleRequestDTO"];
export type MoveFaceRequest = Schemas["dto.MoveFaceRequestDTO"];
export type SetPersonCoverRequest = Schemas["dto.SetPersonCoverRequestDTO"];
export type SetPersonHiddenRequest = Schemas["dto.SetPersonHiddenRequestDTO"];
export type PersonCorrectionResponse = Schemas["dto.PersonCorrectionResponseDTO"];
