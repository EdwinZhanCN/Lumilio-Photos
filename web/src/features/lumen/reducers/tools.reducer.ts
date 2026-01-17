/**
 * Tools Reducer
 * Manages available tools and tool selection
 */

import type { ToolsState, ToolsAction } from "../types";

export const initialToolsState: ToolsState = {
  tools: [],
  loading: false,
  error: null,
  lastFetch: null,
};

export const toolsReducer = (
  state: ToolsState = initialToolsState,
  action: ToolsAction,
): ToolsState => {
  switch (action.type) {
    case "SET_TOOLS": {
      return {
        ...state,
        tools: action.payload,
        loading: false,
        error: null,
        lastFetch: Date.now(),
      };
    }

    case "SET_TOOLS_LOADING": {
      return {
        ...state,
        loading: action.payload,
        error: null,
      };
    }

    case "SET_TOOLS_ERROR": {
      return {
        ...state,
        loading: false,
        error: action.payload,
      };
    }

    case "TOGGLE_TOOL":
    case "CLEAR_SELECTED_TOOLS":
      // These are handled by ui reducer
      return state;

    default:
      return state;
  }
};
