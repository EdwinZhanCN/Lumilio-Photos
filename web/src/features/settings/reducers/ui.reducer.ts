import { SettingsAction, UISettings } from "../settings.type.ts";

export const uiReducer = (
  state: UISettings,
  action: SettingsAction,
): UISettings => {
  switch (action.type) {
    case "SET_THEME_FOLLOW_SYSTEM":
      return {
        ...state,
        theme: {
          ...state.theme,
          followSystem: action.payload,
        },
      };
    case "SET_THEME_MODE":
      return {
        ...state,
        theme: {
          ...state.theme,
          mode: action.payload,
        },
      };
    case "SET_LIGHT_MODE_THEME":
      return {
        ...state,
        theme: {
          ...state.theme,
          themes: {
            ...state.theme.themes,
            light: action.payload,
          },
        },
      };
    case "SET_DARK_MODE_THEME":
      return {
        ...state,
        theme: {
          ...state.theme,
          themes: {
            ...state.theme.themes,
            dark: action.payload,
          },
        },
      };
    case "SET_ASSETS_LAYOUT":
      return {
        ...state,
        asset_page: {
          layout: action.payload,
          columns: state.asset_page?.columns ?? 6,
        },
      };
    case "SET_ASSETS_COLUMNS":
      return {
        ...state,
        asset_page: {
          layout: state.asset_page?.layout ?? "full",
          columns: action.payload,
        },
      };
    case "SET_LANGUAGE":
      return { ...state, language: action.payload };
    case "SET_REGION":
      return { ...state, region: action.payload };
    case "SET_WORKING_REPOSITORY_ID":
      return {
        ...state,
        working_repository_id: action.payload || undefined,
      };
    default:
      return state;
  }
};
