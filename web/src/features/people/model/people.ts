import type { components } from "@/lib/http-commons/schema.d.ts";

type PeopleSchemas = components["schemas"];

export type PersonSummary = PeopleSchemas["dto.PersonSummaryDTO"];
export type PersonDetail = PeopleSchemas["dto.PersonDetailDTO"];
export type ListPeopleResponse = PeopleSchemas["dto.ListPeopleResponseDTO"];
export type FaceClusterRebuildResponse = PeopleSchemas["dto.FaceClusterRebuildResponseDTO"];
export type PersonSummaryList = NonNullable<ListPeopleResponse["people"]>;
export type UpdatePersonRequest = PeopleSchemas["dto.UpdatePersonRequestDTO"];

export type PersonFace = PeopleSchemas["dto.PersonFaceDTO"];
export type ListPersonFacesResponse = PeopleSchemas["dto.ListPersonFacesResponseDTO"];
export type PersonFaceList = NonNullable<ListPersonFacesResponse["faces"]>;
export type MergePeopleRequest = PeopleSchemas["dto.MergePeopleRequestDTO"];
export type MoveFaceRequest = PeopleSchemas["dto.MoveFaceRequestDTO"];
export type SetPersonCoverRequest = PeopleSchemas["dto.SetPersonCoverRequestDTO"];
export type SetPersonHiddenRequest = PeopleSchemas["dto.SetPersonHiddenRequestDTO"];
export type PersonCorrectionResponse = PeopleSchemas["dto.PersonCorrectionResponseDTO"];
