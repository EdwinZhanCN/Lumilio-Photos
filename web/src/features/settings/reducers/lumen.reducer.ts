import { LumenSettings, SettingsAction } from "../types";

export const lumenReducer = (
  state: LumenSettings,
  action: SettingsAction,
): LumenSettings => {
  switch (action.type) {
    case "SET_LUMEN_MODEL":
      return { ...state, model: action.payload };
    case "SET_LUMEN_TEMPERATURE":
      return { ...state, temperature: action.payload };
    case "SET_LUMEN_TOP_P":
      return { ...state, top_p: action.payload };
    case "SET_LUMEN_MODELRECORDS":
      return { ...state, modelRecords: action.payload };
    case "SET_LUMEN_SYSTEM_PROMPT":
      return { ...state, systemPrompt: action.payload };
    case "SET_LUMEN_ENABLED":
      return { ...state, enabled: action.payload };
    default:
      return state;
  }
};
