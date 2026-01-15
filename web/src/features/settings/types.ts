import { ModelRecord } from "@mlc-ai/web-llm";

export interface LumenSettings {
  model: string;
  temperature: number;
  top_p: number;
  modelRecords?: ModelRecord[];
  systemPrompt?: string;
  enabled?: boolean;
}

export interface UISettings {
  language?: "en" | "zh";
  region?: "china" | "other";
  asset_page?: {
    layout: "compact" | "wide" | "full";
  };
  upload?: {
    max_preview_count: number; // 生成预览图的最大数量
    max_total_files: number; // 总文件上传数量限制
    low_power_mode?: boolean; // 低功耗模式开关
    chunk_size_mb?: number; // 客户端分片大小（MB）
    max_concurrent_chunks?: number; // 分片并发上限
    use_server_config?: boolean; // 是否采用后端返回的上传配置
  };
}

export interface ServerSettings {
  update_timespan: number;
}

export interface SettingsState {
  lumen: LumenSettings;
  ui: UISettings;
  server: ServerSettings;
}

export type SettingsAction =
  // Lumen Actions
  | { type: "SET_LUMEN_MODEL"; payload: string }
  | { type: "SET_LUMEN_TEMPERATURE"; payload: number }
  | { type: "SET_LUMEN_TOP_P"; payload: number }
  | { type: "SET_LUMEN_MODELRECORDS"; payload: ModelRecord[] }
  | { type: "SET_LUMEN_SYSTEM_PROMPT"; payload: string }
  | { type: "SET_LUMEN_ENABLED"; payload: boolean }
  // UI Actions
  | { type: "SET_ASSETS_LAYOUT"; payload: "compact" | "wide" | "full" }
  | { type: "SET_LANGUAGE"; payload: "en" | "zh" }
  | { type: "SET_REGION"; payload: "china" | "other" }
  // Upload Actions
  | { type: "SET_UPLOAD_MAX_PREVIEW_COUNT"; payload: number }
  | { type: "SET_UPLOAD_MAX_TOTAL_FILES"; payload: number }
  | { type: "SET_UPLOAD_LOW_POWER_MODE"; payload: boolean }
  | { type: "SET_UPLOAD_CHUNK_SIZE_MB"; payload: number }
  | { type: "SET_UPLOAD_MAX_CONCURRENT_CHUNKS"; payload: number }
  | { type: "SET_UPLOAD_USE_SERVER_CONFIG"; payload: boolean }
  // Server Actions
  | { type: "SET_SERVER_UPDATE_TIMESPAN"; payload: number };

export interface SettingsContextValue {
  state: SettingsState;
  dispatch: React.Dispatch<SettingsAction>;
}
