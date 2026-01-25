// src/services/statsService.ts

import client from "@/lib/http-commons/client";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];

// ==================== Stats Types ====================

export type FocalLengthDistributionResponse =
  Schemas["handler.FocalLengthDistributionResponse"];
export type CameraLensStatsResponse = Schemas["handler.CameraLensStatsResponse"];
export type TimeDistributionResponse = Schemas["handler.TimeDistributionResponse"];
export type HeatmapResponse = Schemas["handler.HeatmapResponse"];
export type AvailableYearsResponse = Schemas["handler.AvailableYearsResponse"];
export type TimeDistributionType = "hourly" | "monthly";

// Re-export bucket types for consumers
export type FocalLengthBucket = Schemas["handler.FocalLengthBucket"];
export type CameraLensCombination = Schemas["handler.CameraLensCombination"];
export type TimeBucket = Schemas["handler.TimeBucket"];
export type HeatmapValue = Schemas["handler.HeatmapValue"];

// Legacy API Result wrapper type for backwards compatibility
export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

// ==================== Stats Service (Direct API calls) ====================

export const statsService = {
  /**
   * Get focal length distribution statistics
   */
  async getFocalLengthDistribution() {
    return client.GET("/api/v1/stats/focal-length", {});
  },

  /**
   * Get camera+lens combination statistics (Top N)
   */
  async getCameraLensStats(limit: number = 20) {
    return client.GET("/api/v1/stats/camera-lens", {
      params: { query: { limit } },
    });
  },

  /**
   * Get shooting time distribution statistics
   */
  async getTimeDistribution(type: TimeDistributionType = "hourly") {
    return client.GET("/api/v1/stats/time-distribution", {
      params: { query: { type } },
    });
  },

  /**
   * Get daily activity heatmap data
   */
  async getDailyActivityHeatmap(days: number = 365) {
    return client.GET("/api/v1/stats/daily-activity", {
      params: { query: { days } },
    });
  },

  /**
   * Get available years with photo data
   */
  async getAvailableYears() {
    return client.GET("/api/v1/stats/available-years", {});
  },
};

// ==================== React Query Hooks ====================

/**
 * Hook for focal length distribution
 */
export const useFocalLengthDistribution = () =>
  $api.useQuery("get", "/api/v1/stats/focal-length", {});

/**
 * Hook for camera+lens stats
 */
export const useCameraLensStats = (limit: number = 20) =>
  $api.useQuery("get", "/api/v1/stats/camera-lens", {
    params: { query: { limit } },
  });

/**
 * Hook for time distribution
 */
export const useTimeDistribution = (type: TimeDistributionType = "hourly") =>
  $api.useQuery("get", "/api/v1/stats/time-distribution", {
    params: { query: { type } },
  });

/**
 * Hook for daily activity heatmap
 */
export const useDailyActivityHeatmap = (days: number = 365) =>
  $api.useQuery("get", "/api/v1/stats/daily-activity", {
    params: { query: { days } },
  });

/**
 * Hook for available years
 */
export const useAvailableYears = () =>
  $api.useQuery("get", "/api/v1/stats/available-years", {});
