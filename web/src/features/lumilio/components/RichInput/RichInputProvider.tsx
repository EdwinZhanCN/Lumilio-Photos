// Lumilio-Photos/web/src/features/lumilio/rich-input/RichInputProvider.tsx
import React, { createContext, useReducer, ReactNode } from "react";
import { RichInputContextValue } from "./types";
import { RichInputReducer, initialState } from "./rich.reducer";

export const RichInputContext = createContext<
  RichInputContextValue | undefined
>(undefined);

/** Context provider component for RichInput state management.
 *
 * Provides the RichInput state and dispatch function to all child components.
 * Uses a reducer pattern to manage the complex state of the rich input editor,
 * including mention/type selection, menu positioning, and payload parsing.
 *
 * @param children - React nodes to be wrapped by the provider.
 *
 * @example
 * ```tsx
 * <RichInputProvider>
 *   <YourComponent />
 * </RichInputProvider>
 * ```
 */
export const RichInputProvider = ({ children }: { children: ReactNode }) => {
  const [state, dispatch] = useReducer(RichInputReducer, initialState);

  const value: RichInputContextValue = {
    state,
    dispatch,
  };

  return (
    <RichInputContext.Provider value={value}>
      {children}
    </RichInputContext.Provider>
  );
};

/** Custom hook to access RichInput context state and dispatch.
 *
 * Provides access to the RichInput editor state including the current phase,
 * active mention type, menu position, selected index, available options,
 * and parsed payload. Also provides the dispatch function to trigger state
 * updates.
 *
 * @returns The RichInputContextValue containing state and dispatch.
 * @throws Error if the hook is used outside of a RichInputProvider.
 *
 * @example
 * ```tsx
 * const { state, dispatch } = useRichInput();
 *
 * // Access state
 * console.log(state.phase);
 * console.log(state.payload);
 *
 * // Dispatch actions
 * dispatch({ type: "SET_PHASE", payload: "SELECT_TYPE" });
 * ```
 */
export const useRichInput = (): RichInputContextValue => {
  const context = React.useContext(RichInputContext);

  if (context === undefined) {
    throw new Error(
      "useRichInput must be used within a RichInputProvider. " +
        "Wrap your component tree with <RichInputProvider>.",
    );
  }

  return context;
};
