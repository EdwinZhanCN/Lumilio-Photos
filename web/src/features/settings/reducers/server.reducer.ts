import { ServerSettings, SettingsAction } from "../types";

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
