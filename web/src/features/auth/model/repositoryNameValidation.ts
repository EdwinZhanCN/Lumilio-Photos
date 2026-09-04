import type { RepositoryNameError } from "@/features/repositories";
import type { useI18n } from "@/lib/i18n.tsx";

type TranslateFn = ReturnType<typeof useI18n>["t"];

export function repositoryNameErrorMessage(error: RepositoryNameError, t: TranslateFn): string {
  switch (error) {
    case "required":
      return t("manage.repositories.createNameRequired", "Enter a Repository name.");
    case "leadingOrTrailingSpace":
      return t(
        "manage.repositories.createNameEdgeSpace",
        "The name cannot start or end with a space.",
      );
    case "tooManyCharacters":
      return t(
        "manage.repositories.createNameTooManyCharacters",
        "The name cannot exceed 80 characters.",
      );
    case "tooManyBytes":
      return t(
        "manage.repositories.createNameTooManyBytes",
        "The name cannot exceed 240 UTF-8 bytes.",
      );
    case "controlCharacter":
      return t(
        "manage.repositories.createNameControlCharacter",
        "The name cannot contain control characters.",
      );
  }
}
