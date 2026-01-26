import { SettingsAction, SettingsState } from "./settings.type.ts";
import { uiReducer } from "./reducers/ui.reducer";
import { serverReducer } from "./reducers/server.reducer";
import { getCurrentLanguage } from "@/lib/i18n.tsx";

const defaultLanguage = getCurrentLanguage();

export const initialState: SettingsState = {
  ui: {
    language: defaultLanguage,
    region: "other",
    asset_page: {
      layout: "full",
    },
    upload: {
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
    ui: uiReducer(state.ui, action),
    server: serverReducer(state.server, action),
  };
};
