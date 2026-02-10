/**
 * OpenAPI React Query Integration
 *
 * Provides typed React Query hooks for API endpoints.
 *
 * Usage:
 * ```tsx
 * // In a component
 * const { data, isLoading } = $api.useQuery("get", "/api/v1/stats/focal-length");
 *
 * // For mutations
 * const mutation = $api.useMutation("post", "/api/v1/auth/login");
 * ```
 */
import createClient from "@/lib/http-commons/openapi-react-query";
import type { paths } from "./schema";
import client from "./client";

export const $api = createClient<paths>(client);
export { client };

export default $api;
