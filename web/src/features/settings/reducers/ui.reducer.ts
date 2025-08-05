import { SettingsAction, UISettings } from "../types";

export const uiReducer = (
  state: UISettings,
  action: SettingsAction,
): UISettings => {
  switch (action.type) {
    case "SET_ASSETS_LAYOUT":
      return { ...state, asset_page: { layout: action.payload } };
    default:
      return state;
  }
};
