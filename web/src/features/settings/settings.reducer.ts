import { SettingsAction, SettingsState } from "./settings.type.ts";
import { uiReducer } from "./reducers/ui.reducer";
import { serverReducer } from "./reducers/server.reducer";
import { getCurrentLanguage } from "@/lib/i18n.tsx";
import { DEFAULT_THEME_PREFERENCES } from "@/lib/theme/daisyuiThemes";

const defaultLanguage = getCurrentLanguage();

export const initialState: SettingsState = {
  ui: {
    language: defaultLanguage,
    region: "other",
    working_repository_id: undefined,
    theme: {
      followSystem: DEFAULT_THEME_PREFERENCES.followSystem,
      mode: DEFAULT_THEME_PREFERENCES.mode,
      themes: {
        light: DEFAULT_THEME_PREFERENCES.themes.light,
        dark: DEFAULT_THEME_PREFERENCES.themes.dark,
      },
    },
    asset_page: {
      layout: "full",
      columns: 6,
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
