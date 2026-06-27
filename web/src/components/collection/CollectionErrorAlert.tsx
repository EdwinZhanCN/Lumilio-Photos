import type { ReactNode } from "react";
import { AlertTriangle } from "lucide-react";

/**
 * The warning alert shown when a collection list fails to load. Replaces the
 * identical `alert alert-warning` block previously duplicated across
 * Collections and People.
 */
export function CollectionErrorAlert({ message }: { message: ReactNode }): ReactNode {
  return (
    <div className="alert alert-warning">
      <AlertTriangle className="size-5" />
      <span>{message}</span>
    </div>
  );
}

export default CollectionErrorAlert;
