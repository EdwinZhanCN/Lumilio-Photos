import { useEffect } from "react";
import { useContextStore } from "@/lib/assistant";
import { useI18n } from "@/lib/i18n.tsx";
import type { BrowseItem } from "../types";
import { resolveBrowseSelectedAssetIds } from "../utils/browseItems";
import { useSelection } from "./useSelection";

const CONTRIBUTOR_ID = "gallery-selection";

/** Registers gallery selection as agent context when selection mode is active. */
export function useGalleryContextContributor(browseItems: BrowseItem[] = []) {
  const { t } = useI18n();
  const { enabled, hasSelection, selectedAsArray } = useSelection();
  const register = useContextStore((s) => s.register);
  const unregister = useContextStore((s) => s.unregister);

  useEffect(() => {
    const assetIds =
      browseItems.length > 0
        ? resolveBrowseSelectedAssetIds(selectedAsArray, browseItems)
        : selectedAsArray
            .map((id) => id.replace(/^asset:/, ""))
            .filter((id) => !id.startsWith("stack:"));

    if (!enabled || !hasSelection || assetIds.length === 0) {
      unregister(CONTRIBUTOR_ID);
      return;
    }

    register({
      id: CONTRIBUTOR_ID,
      type: "selection",
      assetIds,
      label: t("lumilio.context.selected", "{{count}} selected", {
        count: assetIds.length,
      }),
    });

    return () => unregister(CONTRIBUTOR_ID);
  }, [browseItems, enabled, hasSelection, selectedAsArray, register, unregister, t]);
}
