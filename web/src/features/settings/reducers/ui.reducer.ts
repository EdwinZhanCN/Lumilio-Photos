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
    case "SET_UPLOAD_MAX_PREVIEW_COUNT":
      return {
        ...state,
        upload: { ...state.upload!, max_preview_count: action.payload },
      };
    case "SET_UPLOAD_MAX_TOTAL_FILES":
      return {
        ...state,
        upload: { ...state.upload!, max_total_files: action.payload },
      };
    default:
      return state;
  }
};
