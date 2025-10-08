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
  // Server Actions
  | { type: "SET_SERVER_UPDATE_TIMESPAN"; payload: number };

export interface SettingsContextValue {
  state: SettingsState;
  dispatch: React.Dispatch<SettingsAction>;
}
