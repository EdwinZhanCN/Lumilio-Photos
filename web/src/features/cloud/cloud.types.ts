import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];

export type CloudCredential = Schemas["dto.CloudCredentialDTO"];
export type CloudProvider = Schemas["dto.CloudProviderDTO"];
export type CloudAuthChallenge = Schemas["dto.CloudAuthChallengeDTO"];
export type CloudProviderField = Schemas["dto.CloudProviderFieldDTO"];
export type CloudImportRun = Schemas["dto.CloudImportRunDTO"];
export type RepositoryCloudStatus = Schemas["dto.RepositoryCloudStatusDTO"];
