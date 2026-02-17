import { useEffect, useState } from "react";
import { fetchPluginCatalog } from "@/features/studio/plugins/registryClient";
import type {
  CatalogPluginSummary,
  StudioPluginPanel,
} from "@/features/studio/plugins/types";

export interface UseStudioPluginCatalogResult {
  catalog: CatalogPluginSummary[];
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

export function useStudioPluginCatalog(
  panel: StudioPluginPanel,
  enabled = true,
): UseStudioPluginCatalogResult {
  const [catalog, setCatalog] = useState<CatalogPluginSummary[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    if (!enabled) {
      setCatalog([]);
      setError(null);
      return;
    }

    setIsLoading(true);
    try {
      const items = await fetchPluginCatalog(panel);
      setCatalog(items);
      setError(null);
    } catch (err) {
      setCatalog([]);
      setError(err instanceof Error ? err.message : "Failed to load plugin catalog");
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    load().catch(() => {
      // handled in load()
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [panel, enabled]);

  return {
    catalog,
    isLoading,
    error,
    refresh: load,
  };
}
