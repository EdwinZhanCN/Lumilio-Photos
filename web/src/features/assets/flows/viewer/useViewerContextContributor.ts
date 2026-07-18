import { useEffect } from "react";
import { useContextStore } from "@/lib/assistant";
import { useI18n } from "@/lib/i18n.tsx";

const CONTRIBUTOR_ID = "carousel-viewing";

/** Registers the currently viewed carousel asset as agent context. The carousel
 * only mounts while open, so presence of an assetId is sufficient. */
export function useViewerContextContributor(assetId: string | undefined) {
  const { t } = useI18n();
  const register = useContextStore((s) => s.register);
  const unregister = useContextStore((s) => s.unregister);

  useEffect(() => {
    if (!assetId) {
      unregister(CONTRIBUTOR_ID);
      return;
    }

    register({
      id: CONTRIBUTOR_ID,
      type: "viewing",
      assetIds: [assetId],
      label: t("lumilio.context.viewing", "Viewing photo"),
    });
    return () => unregister(CONTRIBUTOR_ID);
  }, [assetId, register, unregister, t]);
}
