import type { RepositoryOption } from "../types.ts";

type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

export function getRepositoryDisplayName(
  repository: RepositoryOption | undefined,
  t: TranslateFn,
): string {
  if (!repository) {
    return t("navbar.repository.all", {
      defaultValue: "All repositories",
    });
  }

  if (repository.isPrimary) {
    return t("navbar.repository.primary", {
      defaultValue: "Primary",
    });
  }

  return repository.name || repository.path;
}
