/**
 * # Cloud
 *
 * Cloud is a small server-state capability shared by Settings, Repositories,
 * and Manage. It intentionally has no route, flow UI, Context, or client store.
 *
 * {@link useCloudProviders} and {@link useCloudCredentials} expose provider
 * metadata and authenticated credentials. Credential create, challenge,
 * reconnect, disconnect, and removal mutations stay beside those queries and
 * invalidate the credential list after successful changes.
 *
 * {@link useRepositoryCloudStatus} reports the cloud binding and latest import
 * run for one repository. {@link useStartRepositoryCloudImport} starts an
 * explicit import and invalidates both repository cloud status and asset-facing
 * queries when the request succeeds.
 *
 * Cloud DTO aliases remain in the feature-wide `types.ts` because both API
 * files and several external feature consumers share the same generated
 * OpenAPI values. The feature does not cast or mirror server responses.
 *
 * @module
 */
import type {
  useCloudCredentials,
  useCloudProviders,
} from "./api/useCloudCredentials.ts";
import type {
  useRepositoryCloudStatus,
  useStartRepositoryCloudImport,
} from "./api/useRepositoryCloud.ts";

export {};
