import { Copy, Trash2, type LucideIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import type { RailCardTone } from "./RailCard";
import {
  getUtilityClassifierTitle,
  UTILITY_CLASSIFIERS,
} from "../utils/utilityClassifiers";

export type UtilityShortcut = {
  /** Stable identity for React keys. */
  key: string;
  /** Destination route. */
  to: string;
  icon: LucideIcon;
  title: string;
  tone: RailCardTone;
};

/**
 * Single source of truth for the utility shortcuts surfaced both on the
 * Collections page (as a horizontal rail) and the dedicated Utilities page
 * (as a grid). Keeping the list here means the two layouts never diverge.
 */
export function useUtilityShortcuts(): UtilityShortcut[] {
  const { t } = useI18n();
  return [
    {
      key: "duplicates",
      to: "/collections/utilities/duplicates",
      icon: Copy,
      title: t("collections.utilities.duplicates.title"),
      tone: "primary",
    },
    {
      key: "trash",
      to: "/collections/trash",
      icon: Trash2,
      title: t("collections.utilities.trash.title"),
      tone: "warning",
    },
    ...UTILITY_CLASSIFIERS.map((classifier): UtilityShortcut => ({
      key: classifier.slug,
      to: `/collections/utilities/${classifier.slug}`,
      icon: classifier.icon,
      title: getUtilityClassifierTitle(t, classifier.slug),
      tone: "accent",
    })),
  ];
}
