import { $api } from "@/lib/http-commons/queryClient";
import type { UseQueryResult } from "@tanstack/react-query";
import type {
  ApiResult,
  UploadConfigResponse,
  UploadProgressResponse,
} from "@/lib/upload/types";

/**
 * React Query hook for fetching upload configuration.
 * 
 * Retrieves server-side upload settings including limits, allowed file types,
 * chunk sizes, and other upload-related configuration.
 * 
 * @returns Query result containing upload configuration
 * 
 * @example
 * ```typescript
 * const { data: config, isLoading, error } = useUploadConfig();
 * 
 * if (config?.data) {
 *   console.log('Max file size:', config.data.maxFileSize);
 *   console.log('Allowed types:', config.data.allowedTypes);
 * }
 * ```
 */
export const useUploadConfig = () =>
  $api.useQuery("get", "/api/v1/assets/batch/config", {}) as UseQueryResult<
    ApiResult<UploadConfigResponse>,
    unknown
  >;

/**
 * React Query hook for fetching upload progress for multiple sessions.
 * 
 * Monitors the progress of ongoing upload sessions, providing real-time
 * updates on upload status, completion percentages, and any errors.
 * 
 * @param sessionIds - Comma-separated session IDs to monitor (optional)
 * @param options - Query configuration options
 * @param options.enabled - Whether the query should be enabled (default: true if sessionIds provided)
 * @param options.refetchInterval - Refetch interval in milliseconds for real-time updates
 * 
 * @returns Query result containing upload progress information
 * 
 * @example
 * ```typescript
 * // Monitor specific sessions
 * const { data: progress } = useUploadProgress('session1,session2', {
 *   refetchInterval: 1000 // Update every second
 * });
 * 
 * // Monitor all active sessions
 * const { data: allProgress } = useUploadProgress();
 * ```
 */
export const useUploadProgress = (
  sessionIds?: string,
  options?: { enabled?: boolean; refetchInterval?: number | false },
) =>
  $api.useQuery(
    "get",
    "/api/v1/assets/batch/progress",
    {
      params: {
        query: sessionIds ? { session_ids: sessionIds } : undefined,
      },
    },
    {
      enabled: options?.enabled ?? Boolean(sessionIds),
      refetchInterval: options?.refetchInterval,
    },
  ) as UseQueryResult<ApiResult<UploadProgressResponse>, unknown>;
