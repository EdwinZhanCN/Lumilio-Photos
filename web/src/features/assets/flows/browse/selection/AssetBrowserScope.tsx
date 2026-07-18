import { createContext, type ReactNode, useCallback, useContext, useMemo, useRef } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import {
  AssetSelectionStoreContext,
  createAssetSelectionStore,
  type AssetSelectionInitialState,
  type AssetSelectionStoreApi,
} from "./selection.store";

interface AssetBrowserNavigation {
  openViewer: (assetId: string) => void;
  replaceViewerAsset: (assetId: string) => void;
  closeViewer: () => void;
}

const AssetBrowserNavigationContext = createContext<AssetBrowserNavigation | undefined>(undefined);

interface AssetBrowserScopeProps {
  children: ReactNode;
  scopeId: string;
  basePath?: string;
  defaultSelectionMode?: "single" | "multiple";
  initialSelection?: AssetSelectionInitialState;
}

/** Creates an isolated selection and viewer-navigation scope for one asset browser. */
export function AssetBrowserScope({
  children,
  scopeId,
  basePath = "/assets",
  defaultSelectionMode = "multiple",
  initialSelection,
}: AssetBrowserScopeProps) {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const storeRef = useRef<{ scopeId: string; store: AssetSelectionStoreApi } | null>(null);

  if (storeRef.current === null || storeRef.current.scopeId !== scopeId) {
    storeRef.current = {
      scopeId,
      store: createAssetSelectionStore({
        selectionMode: defaultSelectionMode,
        ...initialSelection,
      }),
    };
  }

  const navigateTo = useCallback(
    (path: string, replace: boolean) => {
      const query = searchParams.toString();
      void navigate(`${path}${query ? `?${query}` : ""}`, { replace });
    },
    [navigate, searchParams],
  );

  const navigation = useMemo<AssetBrowserNavigation>(
    () => ({
      openViewer: (assetId) => navigateTo(`${basePath}/${assetId}`, false),
      replaceViewerAsset: (assetId) => navigateTo(`${basePath}/${assetId}`, true),
      closeViewer: () => navigateTo(basePath, true),
    }),
    [basePath, navigateTo],
  );

  return (
    <AssetSelectionStoreContext.Provider value={storeRef.current.store}>
      <AssetBrowserNavigationContext.Provider value={navigation}>
        {children}
      </AssetBrowserNavigationContext.Provider>
    </AssetSelectionStoreContext.Provider>
  );
}

export function useAssetBrowserNavigation(): AssetBrowserNavigation {
  const context = useContext(AssetBrowserNavigationContext);
  if (!context) {
    throw new Error("Asset navigation must be used within an AssetBrowserScope");
  }
  return context;
}
