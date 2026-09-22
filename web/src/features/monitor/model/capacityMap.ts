import type { StorageDiagnostic } from "@/features/repositories";

/** Partition a filesystem snapshot, without implementing write admission. */
export function capacityMap(item: StorageDiagnostic) {
  if (!item.capacity_known || !item.total_bytes || item.total_bytes <= 0) return undefined;
  const total = item.total_bytes;
  const available = Math.min(total, Math.max(0, item.available_bytes ?? 0));
  const used = total - available;
  const writable = Math.min(available, Math.max(0, item.writable_budget_bytes ?? 0));
  // The Server's budget already subtracts the reserve. Bound the pictured
  // reserve by actual space, retaining the configured target in the detail.
  const reserve = available - writable;
  return { total, used, available, writable, reserve };
}
