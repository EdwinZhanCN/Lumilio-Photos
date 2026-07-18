export type StorageStrategy = "cas" | "date" | "flat";
export type DuplicateHandling = "overwrite" | "rename" | "uuid";

export function isStorageStrategy(value?: string): value is StorageStrategy {
  return value === "cas" || value === "date" || value === "flat";
}

export function isDuplicateHandling(value?: string): value is DuplicateHandling {
  return value === "overwrite" || value === "rename" || value === "uuid";
}
