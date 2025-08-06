/**
 * @fileoverview Assets Fetch Context Provider for managing asset browsing and filtering
 *
 * This module provides a React context for handling asset fetching operations including
 * infinite scrolling, filtering, searching, and pagination. It follows a performance-optimized
 * pattern by separating state from actions to minimize unnecessary re-renders.
 *
 * The core business logic is encapsulated within the `useFetchProcess` custom hook,
 * making this context a thin wrapper that provides the data to components.
 *
 * @author Edwin Zhan
 * @since 1.0.0
 *
 * @example
 * ```tsx
 * // Wrap your app with the AssetsProvider
 * function App() {
 *   return (
 *     <AssetsProvider>
 *       <AssetBrowser />
 *       <FilterPanel />
 *     </AssetsProvider>
 *   );
 * }
 *
 * // Use the context in your components
 * function AssetGrid() {
 *   const { assets, isLoading, fetchNextPage, hasMore } = useAssetsContext();
 *
 *   return (
 *     <div>
 *       {assets.map(asset => <AssetCard key={asset.assetId} asset={asset} />)}
 *       {hasMore && <button onClick={fetchNextPage}>Load More</button>}
 *     </div>
 *   );
 * }
 * ```
 *
 * @todo Add support for real-time asset updates via WebSocket
 */
import { createContext, ReactNode } from "react";
import { useFetchProcess } from "@/hooks/api-hooks/useFetchProcess";
import { AssetsState, AssetsActions } from "./types";

/**
 * **Assets State Context**
 *
 * React context for sharing asset state across components.
 * Separated from actions context for performance optimization.
 *
 * @internal
 */
export const AssetsStateContext = createContext<AssetsState | undefined>(
  undefined,
);

/**
 * **Assets Actions Context**
 *
 * React context for sharing asset actions across components.
 * Separated from state context to prevent unnecessary re-renders.
 *
 * @internal
 */
export const AssetsActionsContext = createContext<AssetsActions | undefined>(
  undefined,
);

/**
 * **Assets Provider Props**
 *
 * Props interface for the AssetsProvider component.
 *
 * @interface AssetsProviderProps
 */
interface AssetsProviderProps {
  /** Child components that will have access to the assets context */
  children: ReactNode;
}

/**
 * **Assets Provider Component**
 *
 * Main provider component that manages asset fetching state and provides context to child components.
 * Uses a performance-optimized pattern with separate state and actions contexts.
 *
 * @param props - Provider props containing children
 * @returns JSX element wrapping children with assets context
 *
 * @since 1.0.0
 */
export function AssetsProvider({ children }: AssetsProviderProps) {
  const { state, actions } = useFetchProcess();

  return (
    <AssetsActionsContext.Provider value={actions}>
      <AssetsStateContext.Provider value={state}>
        {children}
      </AssetsStateContext.Provider>
    </AssetsActionsContext.Provider>
  );
}
