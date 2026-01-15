import { ModelRecord } from "@mlc-ai/web-llm";
import { SettingsAction, SettingsState } from "./types";
import { lumenReducer } from "./reducers/lumen.reducer";
import { uiReducer } from "./reducers/ui.reducer";
import { serverReducer } from "./reducers/server.reducer";
import { getCurrentLanguage } from "@/lib/i18n.tsx";

export const defaultModelRecords: ModelRecord[] = [
  {
    model: "https://huggingface.co/mlc-ai/Qwen3-4B-q4f16_1-MLC",
    model_id: "Qwen3-4B-q4f16_1-MLC",
    model_lib:
      "https://raw.githubusercontent.com/mlc-ai/binary-mlc-llm-libs/main/web-llm-models/v0_2_48/Qwen3-4B-q4f16_1-ctx4k_cs1k-webgpu.wasm",
    vram_required_MB: 3431.59,
    low_resource_required: true,
    overrides: {
      context_window_size: 4096,
    },
  },
  {
    model: "https://huggingface.co/mlc-ai/Qwen3-1.7B-q4f16_1-MLC",
    model_id: "Qwen3-1.7B-q4f16_1-MLC",
    model_lib:
      "https://raw.githubusercontent.com/mlc-ai/binary-mlc-llm-libs/main/web-llm-models/v0_2_48/Qwen3-1.7B-q4f16_1-ctx4k_cs1k-webgpu.wasm",
    vram_required_MB: 2036.66,
    low_resource_required: true,
    overrides: {
      context_window_size: 4096,
    },
  },
];

const defaultLanguage = getCurrentLanguage();

export const initialState: SettingsState = {
  lumen: {
    model: "Qwen3-1.7B-q4f16_1-MLC",
    temperature: 0.8,
    top_p: 0.95,
    modelRecords: defaultModelRecords,
    systemPrompt:
      "You are a helpful AI assistant that provides informative and concise responses about various topics. Be friendly and engaging in your responses.",
    enabled: true,
  },
  ui: {
    language: defaultLanguage,
    region: "other",
    asset_page: {
      layout: "full",
    },
    upload: {
      max_preview_count: 30, // 默认生成30个预览图
      max_total_files: 100, // 默认最多上传100个文件
      low_power_mode: true, // 默认开启低功耗上传
      chunk_size_mb: 24, // 默认分片大小 24MB
      max_concurrent_chunks: 2, // 默认分片并发 2
      use_server_config: true, // 默认读取后端上传配置
    },
  },
  server: {
    update_timespan: 5,
  },
};

export const SettingsReducer = (
  state: SettingsState,
  action: SettingsAction,
): SettingsState => {
  return {
    lumen: lumenReducer(state.lumen, action),
    ui: uiReducer(state.ui, action),
    server: serverReducer(state.server, action),
  };
};
