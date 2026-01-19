/**
 * UI Reducer
 * Manages UI state for agent interface
 */

import type { AgentUIState, AgentAction } from "../types";

export const initialUIState: AgentUIState = {
  isInputDisabled: false,
  selectedTools: [],
  showWelcome: true,
};

export const uiReducer = (
  state: AgentUIState = initialUIState,
  action: AgentAction,
): AgentUIState => {
  switch (action.type) {
    case "SET_INPUT_DISABLED": {
      return {
        ...state,
        isInputDisabled: (action as any).payload,
      };
    }

    case "SET_SHOW_WELCOME": {
      return {
        ...state,
        showWelcome: (action as any).payload,
      };
    }

    case "TOGGLE_TOOL": {
      const toolName = (action as any).payload;
      const isSelected = state.selectedTools.includes(toolName);

      return {
        ...state,
        selectedTools: isSelected
          ? state.selectedTools.filter((t) => t !== toolName)
          : [...state.selectedTools, toolName],
      };
    }

    case "CLEAR_SELECTED_TOOLS": {
      return {
        ...state,
        selectedTools: [],
      };
    }

    default:
      return state;
  }
};
