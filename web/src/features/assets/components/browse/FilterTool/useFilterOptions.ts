import { useEffect, useState } from "react";
import { useAssetFilterOptions } from "../../../api/useAssetFilterOptions";

interface UseFilterOptionsParams {
  open: boolean;
  cameraModelOptions?: string[];
  lensOptions?: string[];
  fetchCameraModels?: () => Promise<string[]>;
  fetchLenses?: () => Promise<string[]>;
}

export function useFilterOptions({
  open,
  cameraModelOptions,
  lensOptions,
  fetchCameraModels,
  fetchLenses,
}: UseFilterOptionsParams) {
  const [cameraModelItems, setCameraModelItems] = useState<string[]>(cameraModelOptions ?? []);
  const [lensItems, setLensItems] = useState<string[]>(lensOptions ?? []);
  const [isCustomLoading, setIsCustomLoading] = useState(false);
  const [hasLoaded, setHasLoaded] = useState(false);

  const needsCameraModels = !cameraModelOptions || cameraModelOptions.length === 0;
  const needsLenses = !lensOptions || lensOptions.length === 0;
  const needsOptions = needsCameraModels || needsLenses;
  const canUseCustomFetchers = !!fetchCameraModels && !!fetchLenses;
  const shouldFetchDefault = open && !hasLoaded && needsOptions && !canUseCustomFetchers;

  const filterOptionsQuery = useAssetFilterOptions(shouldFetchDefault);
  const loadingOptions = isCustomLoading || filterOptionsQuery.isFetching;

  useEffect(() => {
    const shouldFetch = open && !hasLoaded && needsOptions && canUseCustomFetchers;
    if (!shouldFetch) return;

    let running = true;
    const load = async () => {
      try {
        setIsCustomLoading(true);

        let cm: string[] = cameraModelOptions ?? [];
        let ln: string[] = lensOptions ?? [];

        cm = await fetchCameraModels!();
        ln = await fetchLenses!();

        if (running) {
          setCameraModelItems(cm);
          setLensItems(ln);
          setHasLoaded(true);
        }
      } catch {
        // Option loading is best-effort; the selectors remain usable with no options.
      } finally {
        if (running) setIsCustomLoading(false);
      }
    };
    void load();

    return () => {
      running = false;
    };
  }, [
    open,
    cameraModelOptions,
    lensOptions,
    fetchCameraModels,
    fetchLenses,
    needsOptions,
    canUseCustomFetchers,
    hasLoaded,
  ]);

  useEffect(() => {
    if (!shouldFetchDefault) return;
    const response = filterOptionsQuery.data;
    if (!response) return;

    if (needsCameraModels) setCameraModelItems(response.camera_models || []);
    if (needsLenses) setLensItems(response.lenses || []);
    setHasLoaded(true);
  }, [shouldFetchDefault, filterOptionsQuery.data, needsCameraModels, needsLenses]);

  return { cameraModelItems, lensItems, loadingOptions };
}
