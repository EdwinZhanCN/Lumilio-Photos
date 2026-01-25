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
import createClient from "openapi-react-query";
import type { paths } from "./schema";
import client from "./client";

/**
 * React Query hooks wrapper for typed API calls
 */
export const $api = createClient<paths>(client);

export default $api;
