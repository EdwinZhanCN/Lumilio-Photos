/**
 * Storage label rules for the admin storage view.
 *
 * A Repository's capacity is measured at its own path, so the number is always
 * trustworthy. What a reader cannot guess from the number alone is *which*
 * storage it came from: a Storage Location may hold Repositories on different
 * mounts (a Docker child mount, for example). The server reports the mount
 * point the capacity was sampled from; this module turns that raw host path
 * into something a person recognizes, exactly the way Finder and Explorer name
 * storage.
 *
 * The rule is deliberately storage-shaped, not pool-shaped: grouping (whether
 * two paths are *proven* to share one backing filesystem) stays an accounting
 * concern and never reaches the page.
 */

/** A mount point that is the machine's own system storage rather than a named volume. */
const SYSTEM_MOUNT_PATHS = new Set(["/", "/System/Volumes/Data", "C:\\", "C:"]);

export type StorageLabel =
  /** The machine's own disk; copy is resolved by the caller through i18n. */
  | { kind: "system"; mountPath: string }
  /** A named mount such as /Volumes/Backup or /volume1. */
  | { kind: "named"; mountPath: string; name: string }
  /** The mount could not be resolved for this path. */
  | { kind: "unknown" };

/**
 * Turns a sampled mount point into a displayable storage identity. The server
 * never localizes this value, so the "system" case is left as a marker for the
 * caller to translate.
 */
export function deriveStorageLabel(mountPath?: string | null): StorageLabel {
  const trimmed = (mountPath ?? "").trim();
  if (trimmed === "") return { kind: "unknown" };
  if (SYSTEM_MOUNT_PATHS.has(trimmed)) return { kind: "system", mountPath: trimmed };

  // macOS reaches the data volume through firmlinks such as
  // /System/Volumes/Data and /System/Volumes/Preboot. They are the same
  // internal disk to a reader, so they must not read as three storages.
  if (trimmed === "/System/Volumes" || trimmed.startsWith("/System/Volumes/")) {
    return { kind: "system", mountPath: trimmed };
  }

  const normalized = trimmed.replace(/[\\/]+$/, "");
  const name = normalized.split(/[\\/]/).filter(Boolean).pop();
  if (!name) return { kind: "system", mountPath: trimmed };

  return { kind: "named", mountPath: trimmed, name };
}
