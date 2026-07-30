export type StorageStrategy = "cas" | "date" | "flat";
export type DuplicateHandling = "overwrite" | "rename" | "uuid";
export type RepositoryNameError =
  | "required"
  | "leadingOrTrailingSpace"
  | "tooManyCharacters"
  | "tooManyBytes"
  | "unsupportedCharacter";

const MAX_REPOSITORY_NAME_CHARACTERS = 80;
const MAX_REPOSITORY_NAME_BYTES = 240;
const ALLOWED_REPOSITORY_NAME_CHARACTER = /^[\p{L}\p{Nd} _-]$/u;

export function isStorageStrategy(value?: string): value is StorageStrategy {
  return value === "cas" || value === "date" || value === "flat";
}

export function isDuplicateHandling(value?: string): value is DuplicateHandling {
  return value === "overwrite" || value === "rename" || value === "uuid";
}

export function validateRepositoryName(value: string): RepositoryNameError | null {
  if (value.length === 0) return "required";
  if (new TextEncoder().encode(value).length > MAX_REPOSITORY_NAME_BYTES) {
    return "tooManyBytes";
  }
  if (Array.from(value).length > MAX_REPOSITORY_NAME_CHARACTERS) {
    return "tooManyCharacters";
  }
  if (value.startsWith(" ") || value.endsWith(" ")) {
    return "leadingOrTrailingSpace";
  }
  for (const character of value) {
    if (!ALLOWED_REPOSITORY_NAME_CHARACTER.test(character)) {
      return "unsupportedCharacter";
    }
  }
  return null;
}
