import { RichInputState, RichInputAction, initialState } from "./types";

/** Reducer for managing RichInput component state.
 *
 * Handles all state transitions for the rich input editor, including:
 * - Trigger phase management (IDLE, SELECT_TYPE, SELECT_ENTITY, COMMAND)
 * - Current active mention type
 * - Floating menu positioning
 * - Selected index for keyboard navigation
 * - Available menu options
 * - Parsed payload string
 *
 * @param state - The current RichInput state.
 * @param action - The action to dispatch for state update.
 * @returns The updated RichInput state based on the dispatched action.
 */
export const RichInputReducer = (
  state: RichInputState,
  action: RichInputAction,
): RichInputState => {
  switch (action.type) {
    case "SET_PHASE":
      return {
        ...state,
        phase: action.payload,
      };

    case "SET_ACTIVE_MENTION_TYPE":
      return {
        ...state,
        activeMentionType: action.payload,
      };

    case "SET_MENU_POSITION":
      return {
        ...state,
        menuPos: action.payload,
      };

    case "SET_SELECTED_INDEX":
      return {
        ...state,
        selectedIndex: action.payload,
      };

    case "SET_OPTIONS":
      return {
        ...state,
        options: action.payload,
      };

    case "SET_PAYLOAD":
      return {
        ...state,
        payload: action.payload,
      };

    case "RESET_EDITOR":
      return {
        ...initialState,
      };

    default:
      return state;
  }
};

export type { RichInputAction };
export { initialState };
