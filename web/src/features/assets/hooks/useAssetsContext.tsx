import { useContext } from "react";
import { AssetsContext } from "../AssetsProvider";
import { AssetsContextValue } from "../assets.types.ts";

/**
 * Main hook for accessing the assets context.
 * Provides access to the complete assets state and dispatch function.
 *
 * @returns AssetsContextValue containing state and dispatch
 * @throws Error if used outside of AssetsProvider
 *
 * @example
 * ```tsx
 * function MyComponent() {
 *   const { state, dispatch } = useAssetsContext();
 *
 *   const handleSelectAsset = (assetId: string) => {
 *     dispatch({ type: "SELECT_ASSET", payload: { assetId } });
 *   };
 *
 *   return <div>Current tab: {state.ui.currentTab}</div>;
 * }
 * ```
 */
export const useAssetsContext = (): AssetsContextValue => {
  const context = useContext(AssetsContext);

  if (context === undefined) {
    throw new Error("useAssetsContext must be used within an AssetsProvider");
  }

  return context;
};

/**
 * Hook for accessing navigation helpers from the context.
 * These are convenience methods for common navigation operations.
 *
 * @returns Navigation helper functions
 *
 * @example
 * ```tsx
 * function CarouselButton() {
 *   const { openCarousel, closeCarousel } = useAssetsNavigation();
 *
 *   return (
 *     <button onClick={() => openCarousel('asset-123')}>
 *       Open Asset
 *     </button>
 *   );
 * }
 * ```
 */
export const useAssetsNavigation = () => {
  const { openCarousel, closeCarousel, switchTab } = useAssetsContext();

  return {
    openCarousel,
    closeCarousel,
    switchTab,
  };
};
