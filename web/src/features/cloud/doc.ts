/**
 * # Cloud
 *
 * Cloud is an API capability shared by Settings, Repositories, and Manage. It
 * owns provider descriptors, authenticated cloud credentials, repository
 * bindings, explicit import runs, and the repository source-management flow.
 *
 * ## State
 *
 * Cloud facts remain TanStack Query server state. The feature has no Context,
 * Zustand store, URL state, or browser persistence. Credential mutations
 * invalidate the credential list, and repository import status polls only
 * while its latest run is queued or running.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     SETTINGS["Settings"] --> CREDENTIALS["credential hooks"]
 *     REPOSITORIES["Repositories"] --> SOURCES["CloudSourcesModal"]
 *     SOURCES --> STATUS["repository cloud sources"]
 *     SOURCES --> IMPORT["start / cancel / resume import"]
 *     CREDENTIALS --> API["Cloud API"]
 *     STATUS --> API
 *     IMPORT --> API
 * ```
 *
 * Settings composes provider and credential setup. After repository creation,
 * {@link CloudSourcesModal} binds one or more connected accounts (including a
 * provider-specific remote scope), renders their durable receipts, and starts,
 * cancels, or resumes repository-scoped imports.
 *
 * ## Data
 *
 * {@link useCloudProviders} and {@link useCloudCredentials} read server
 * metadata and connected accounts. Credential creation, challenge,
 * reconnect, disconnect, and removal live beside those queries.
 * {@link useRepositoryCloudStatus} reads every visible binding and latest run.
 * {@link useBindRepositoryCloudSource}, {@link useStartRepositoryCloudImport},
 * {@link useCancelCloudImport}, and {@link useResumeCloudImport} mutate that
 * server-owned state and invalidate repository-facing status.
 *
 * Generated OpenAPI aliases live in `types.ts` because both API modules and
 * consumers share them. {@link createProviderTextResolver} maps backend i18n
 * keys to extractor-visible defaults while allowing future provider keys to
 * degrade to their raw value. The root `index.ts` is the feature's complete
 * public contract.
 *
 * @module
 */
import type { useCloudCredentials, useCloudProviders } from "./api/useCloudCredentials.ts";
import type {
  useBindRepositoryCloudSource,
  useCancelCloudImport,
  useRepositoryCloudStatus,
  useResumeCloudImport,
  useStartRepositoryCloudImport,
} from "./api/useRepositoryCloud.ts";
import type CloudSourcesModal from "./flows/repository-sources/CloudSourcesModal.tsx";
import type { createProviderTextResolver } from "./model/providerText.ts";

export {};
