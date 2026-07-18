/**
 * # Notifications
 *
 * Notifications is the narrow feature facade for the application's global
 * notification runtime. Durable in-session notification records remain owned
 * by `GlobalContext`; this feature does not mirror them in a feature store.
 *
 * ## Surfaces
 *
 * {@link useMessage} is the public command used by product workflows. A call
 * adds one record to the global message center and presents the same event as
 * a Sonner toast. Dismissing the toast marks the matching record as read.
 *
 * {@link MessageCenter} renders the recent notification history in the app
 * navigation and delegates read/clear commands to `GlobalContext`.
 * {@link Notifications} mounts the process-wide {@link Toaster}; it contains
 * no notification data and only selects theme-aware toast presentation.
 *
 * ## Decisions
 *
 * The feature intentionally keeps `components/` and `hooks/` rather than
 * inventing a workflow or state directory. Its UI is reused by app
 * composition, while notification state is a genuinely cross-cutting runtime
 * concern already owned by `GlobalContext`.
 *
 * @module
 */
import type MessageCenter from "./components/MessageCenter.tsx";
import type Notifications from "./components/Notifications.tsx";
import type { Toaster } from "./components/Toaster.tsx";
import type { useMessage } from "./hooks/useMessage.ts";

export {};
