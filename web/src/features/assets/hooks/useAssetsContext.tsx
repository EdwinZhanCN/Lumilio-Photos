import { useContext, useMemo } from "react";
import { AssetsContextValue } from "../types";
import { AssetsStateContext, AssetsActionsContext } from "../AssetsProvider";

/**
 * **Assets Context Hook**
 *
 * Primary hook for components to interact with the assets context.
 * Provides type-safe access to both state and actions with automatic error handling.
 *
 * @returns Combined assets state and actions
 * @throws Error if used outside of AssetsProvider
 *
 * @since 1.0.0
 * @see {@link AssetsProvider} for the context provider
 * @see {@link AssetsContextValue} for the complete API reference
 */
export function useAssetsContext(): AssetsContextValue {
  const state = useContext(AssetsStateContext);
  const actions = useContext(AssetsActionsContext);

  if (state === undefined || actions === undefined) {
    throw new Error("useAssetsContext must be used within an AssetsProvider");
  }

  const contextValue = useMemo(
    () => ({ ...state, ...actions }),
    [state, actions],
  );

  return contextValue;
}
