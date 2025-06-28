import { createContext, useContext, useMemo, ReactNode } from "react";
import { ListAssetsParams } from "@/services/getAssetsService";
import { useFetchProcess } from "@/hooks/api-hooks/useFetchProcess"; // Import the actual hook

// --- JSDOC DOCUMENTATION ---
/**
 * @fileoverview
 * This file defines the React Context for fetching, displaying, and managing assets.
 * It follows a performance-optimized pattern by separating state from actions,
 * making it suitable for features like infinite scrolling, filtering, and searching.
 *
 * The core business logic is encapsulated within the `useFetchProcess` custom hook.
 */

// --- TYPE DEFINITIONS ---

// Define asset-related types. In a real app, these would likely live in a dedicated `types.ts` file.
interface Asset {
  assetId?: string;
  uploadTime?: string;
  originalFilename?: string;
  fileSize?: number;
  tags?: AssetTag[];
  type?: "PHOTO" | "VIDEO" | "AUDIO" | "DOCUMENT";
  thumbnails?: AssetThumbnail[];
  description?: string;
}

interface AssetTag {
  tagId?: number;
  tagName?: string;
}

interface AssetThumbnail {
  size?: "small" | "medium" | "large";
  storagePath?: string;
}

/**
 * @interface AssetsState
 * Defines the shape of the state values managed by the AssetsContext.
 * These values represent the current state of the asset Browse view.
 */
export interface AssetsState {
  /** The array of assets currently displayed. */
  assets: Asset[];
  /** The current filter and search parameters. */
  filters: ListAssetsParams;
  /** True if the initial asset list is being fetched. */
  isLoading: boolean;
  /** True when fetching the next page for infinite scroll. */
  isLoadingNextPage: boolean;
  /** An error message if a fetch operation fails. */
  error: string | null;
  /** True if there are more assets to fetch from the server. */
  hasMore: boolean;
}

/**
 * @interface AssetsActions
 * Defines the functions available to manipulate the assets state.
 * These actions are stable and won't cause re-renders for components that only use them.
 */
export interface AssetsActions {
  /** Fetches the first page of assets based on new parameters, replacing the current list. */
  fetchAssets: (params: ListAssetsParams) => Promise<void>;
  /** Fetches the next page of assets and appends them to the current list. */
  fetchNextPage: () => Promise<void>;
  /** A higher-level function to apply a new filter and refetch the asset list from the start. */
  applyFilter: (key: keyof ListAssetsParams, value: any) => void;
  /** A higher-level function to apply a new search query and refetch the asset list. */
  setSearchQuery: (query: string) => void;
  /** Resets all filters to their default state and refetches. */
  resetFilters: () => void;
}

/**
 * The combined value provided by the `useAssetsContext` hook.
 * @typedef {AssetsState & AssetsActions} AssetsContextValue
 */
type AssetsContextValue = AssetsState & AssetsActions;

// --- CONTEXT CREATION ---

const AssetsStateContext = createContext<AssetsState | undefined>(undefined);
const AssetsActionsContext = createContext<AssetsActions | undefined>(
  undefined,
);

// --- PROVIDER COMPONENT ---

interface AssetsProviderProps {
  children: ReactNode;
}

/**
 * @component AssetsProvider
 * @description
 * Encapsulates the logic for fetching and managing assets. It should wrap any
 * component tree that requires access to the asset list or fetching actions.
 */
export default function AssetsProvider({ children }: AssetsProviderProps) {
  // =================================================================================
  // HOOK INTEGRATION POINT
  // =================================================================================
  const { state, actions } = useFetchProcess();

  // --- CONTEXT VALUE CREATION (MEMOIZED) ---
  const actionsValue = useMemo(() => actions, [actions]);
  const stateValue = useMemo(() => state, [state]);

  return (
    <AssetsActionsContext.Provider value={actionsValue}>
      <AssetsStateContext.Provider value={stateValue}>
        {children}
      </AssetsStateContext.Provider>
    </AssetsActionsContext.Provider>
  );
}

// --- CONSUMER HOOK ---

/**
 * @hook useAssetsContext
 * @description
 * The primary hook for components to interact with the AssetsContext.
 * It provides access to all asset-related state and actions.
 * @returns {AssetsContextValue} The combined state and actions.
 * @throws {Error} If used outside of an `<AssetsProvider>`.
 */
export function useAssetsContext(): AssetsContextValue {
  const state = useContext(AssetsStateContext);
  const actions = useContext(AssetsActionsContext);

  if (state === undefined || actions === undefined) {
    throw new Error("useAssetsContext must be used within an AssetsProvider");
  }

  return { ...state, ...actions };
}
