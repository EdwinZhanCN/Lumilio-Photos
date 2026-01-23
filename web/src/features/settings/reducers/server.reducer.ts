import { ServerSettings, SettingsAction } from "../settings.types.ts";

export const serverReducer = (
  state: ServerSettings,
  action: SettingsAction,
): ServerSettings => {
  switch (action.type) {
    case "SET_SERVER_UPDATE_TIMESPAN":
      return { ...state, update_timespan: action.payload };
    default:
      return state;
  }
};
