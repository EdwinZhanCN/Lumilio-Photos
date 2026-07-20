/** Compact human-readable byte size, matching the Go side's humanBytes. */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return "—";
  const unit = 1024;
  if (bytes < unit) return `${bytes} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let value = bytes / unit;
  let index = 0;
  while (value >= unit && index < units.length - 1) {
    value /= unit;
    index += 1;
  }
  return `${value.toFixed(1)} ${units[index]}`;
}
