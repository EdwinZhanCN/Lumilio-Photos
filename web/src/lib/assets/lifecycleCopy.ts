import type { TFunction } from "i18next";

export const missingLabel = (t: TFunction) => t("assets.lifecycle.missing", "Missing");
export const trashLabel = (t: TFunction) => t("assets.trash.title", "Trash");
export const permanentDeleteLabel = (t: TFunction) =>
  t("assets.lifecycle.deletePermanently", "Delete permanently");
export const removeMissingLabel = (t: TFunction) =>
  t("assets.lifecycle.removeMissing", "Remove missing items");
export const emptyTrashLabel = (t: TFunction) => t("assets.lifecycle.emptyTrash", "Empty trash");
