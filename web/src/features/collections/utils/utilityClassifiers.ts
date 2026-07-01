import { FileText, Palette, ReceiptText, type LucideIcon } from "lucide-react";

export type UtilityClassifierSlug = "documents" | "receipts" | "illustration";

export type UtilityClassifierDefinition = {
  slug: UtilityClassifierSlug;
  tagName: "document" | "receipt" | "illustration";
  icon: LucideIcon;
};

export const UTILITY_CLASSIFIERS: UtilityClassifierDefinition[] = [
  {
    slug: "documents",
    tagName: "document",
    icon: FileText,
  },
  {
    slug: "receipts",
    tagName: "receipt",
    icon: ReceiptText,
  },
  {
    slug: "illustration",
    tagName: "illustration",
    icon: Palette,
  },
];

export function findUtilityClassifier(slug?: string) {
  return UTILITY_CLASSIFIERS.find((classifier) => classifier.slug === slug);
}

export function getUtilityClassifierTitle(t: (key: string) => string, slug: UtilityClassifierSlug) {
  switch (slug) {
    case "documents":
      return t("collections.utilities.classifiers.documents.title");
    case "receipts":
      return t("collections.utilities.classifiers.receipts.title");
    case "illustration":
      return t("collections.utilities.classifiers.illustration.title");
  }
}
