import type { components } from "@/lib/http-commons/schema.d.ts";
import type { AxiosRequestConfig, AxiosResponse } from "axios";
import api from "@/lib/http-commons/api.ts";

type Schemas = components["schemas"];

// API Result wrapper type
export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

// ==================== Stats Types ====================

/**
 * 焦距分布数据项
 */
export interface FocalLengthBucket {
  focal_length: number; // 焦距值（mm）
  count: number; // 照片数量
}

/**
 * 焦距分布响应
 */
export interface FocalLengthDistributionResponse {
  data: FocalLengthBucket[];
  total: number;
}

/**
 * 相机+镜头组合数据项
 */
export interface CameraLensCombination {
  camera_model: string;
  lens_model: string;
  count: number;
}

/**
 * 相机镜头组合统计响应
 */
export interface CameraLensStatsResponse {
  data: CameraLensCombination[];
  total: number;
}

/**
 * 时间分布数据项
 */
export interface TimeBucket {
  label: string; // 时间标签（如 "14:00" 或 "2024-01"）
  value: number; // 小时(0-23)或时间戳
  count: number; // 照片数量
}

/**
 * 时间分布响应
 */
export interface TimeDistributionResponse {
  data: TimeBucket[];
  type: "hourly" | "monthly";
}

/**
 * 时间分布类型
 */
export type TimeDistributionType = "hourly" | "monthly";

/**
 * 热力图数据项
 */
export interface HeatmapValue {
  date: string;
  count: number;
}

/**
 * 热力图响应
 */
export interface HeatmapResponse {
  data: HeatmapValue[];
}

/**
 * 可用年份响应
 */
export interface AvailableYearsResponse {
  years: number[];
}

// ==================== Stats Service ====================

/**
 * 照片统计分析服务
 * 提供焦距分布、相机镜头组合、拍摄时间分布等统计功能
 */
export const statsService = {
  /**
   * 获取焦距分布统计
   * @param config - Axios 请求配置
   * @returns 焦距分布数据
   * @example
   * ```typescript
   * const response = await statsService.getFocalLengthDistribution();
   * const distribution = response.data.data;
   * console.log(`Total photos: ${distribution?.total}`);
   * ```
   */
  getFocalLengthDistribution(
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<FocalLengthDistributionResponse>>> {
    return api.get("/api/v1/stats/focal-length", config);
  },

  /**
   * 获取相机+镜头组合统计（Top N）
   * @param limit - 返回数量限制，默认 20
   * @param config - Axios 请求配置
   * @returns 相机镜头组合数据
   * @example
   * ```typescript
   * const response = await statsService.getCameraLensStats(10);
   * const stats = response.data.data;
   * stats?.data.forEach(combo => {
   *   console.log(`${combo.camera_model} + ${combo.lens_model}: ${combo.count}`);
   * });
   * ```
   */
  getCameraLensStats(
    limit: number = 20,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<CameraLensStatsResponse>>> {
    return api.get("/api/v1/stats/camera-lens", {
      ...config,
      params: {
        limit,
        ...config?.params,
      },
    });
  },

  /**
   * 获取拍摄时间分布统计
   * @param type - 分布类型: "hourly" (按小时) 或 "monthly" (按月份)
   * @param config - Axios 请求配置
   * @returns 时间分布数据
   * @example
   * ```typescript
   * // 获取按小时分布
   * const hourlyResponse = await statsService.getTimeDistribution("hourly");
   *
   * // 获取按月份分布
   * const monthlyResponse = await statsService.getTimeDistribution("monthly");
   * ```
   */
  getTimeDistribution(
    type: TimeDistributionType = "hourly",
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<TimeDistributionResponse>>> {
    return api.get("/api/v1/stats/time-distribution", {
      ...config,
      params: {
        type,
        ...config?.params,
      },
    });
  },

  /**
   * 获取每日拍摄活跃度热力图数据
   * @param days - 回溯天数，默认 365 天
   * @param config - Axios 请求配置
   * @returns 热力图数据
   * @example
   * ```typescript
   * // 获取过去一年的活跃度
   * const response = await statsService.getDailyActivityHeatmap(365);
   * const heatmapData = response.data.data;
   * ```
   */
  getDailyActivityHeatmap(
    days: number = 365,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<HeatmapResponse>>> {
    return api.get("/api/v1/stats/daily-activity", {
      ...config,
      params: {
        days,
        ...config?.params,
      },
    });
  },

  /**
   * 获取有照片数据的可用年份列表
   * @param config - Axios 请求配置
   * @returns 可用年份列表
   * @example
   * ```typescript
   * const response = await statsService.getAvailableYears();
   * const years = response.data.data?.years || [];
   * ```
   */
  getAvailableYears(
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AvailableYearsResponse>>> {
    return api.get("/api/v1/stats/available-years", config);
  },
};
