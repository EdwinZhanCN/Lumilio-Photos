import { SettingsAction, UISettings } from "../types";

export const uiReducer = (
  state: UISettings,
  action: SettingsAction,
): UISettings => {
  switch (action.type) {
    case "SET_ASSETS_LAYOUT":
      return { ...state, asset_page: { layout: action.payload } };
    case "SET_LANGUAGE":
      return { ...state, language: action.payload };
    case "SET_REGION":
      return { ...state, region: action.payload };
    case "SET_UPLOAD_MAX_TOTAL_FILES":
      return {
        ...state,
        upload: { ...state.upload!, max_total_files: action.payload },
      };
    case "SET_UPLOAD_LOW_POWER_MODE":
      return {
        ...state,
        upload: { ...state.upload!, low_power_mode: action.payload },
      };
    case "SET_UPLOAD_CHUNK_SIZE_MB":
      return {
        ...state,
        upload: { ...state.upload!, chunk_size_mb: action.payload },
      };
    case "SET_UPLOAD_MAX_CONCURRENT_CHUNKS":
      return {
        ...state,
        upload: { ...state.upload!, max_concurrent_chunks: action.payload },
      };
    case "SET_UPLOAD_USE_SERVER_CONFIG":
      return {
        ...state,
        upload: { ...state.upload!, use_server_config: action.payload },
      };
    default:
      return state;
  }
};
