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
  asset_page?: {
    layout: "compact" | "wide" | "full";
  };
}

export interface SettingsState {
  lumen: LumenSettings;
  ui: UISettings;
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
  | { type: "SET_LANGUAGE"; payload: "en" | "zh" };

export interface SettingsContextValue {
  state: SettingsState;
  dispatch: React.Dispatch<SettingsAction>;
}
