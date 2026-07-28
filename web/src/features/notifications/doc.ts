/**
 * # Notifications
 *
 * Notifications is the narrow feature facade for application-wide
 * notifications. It exposes product commands and navigation surfaces while
 * the cross-cutting notification runtime remains owned by `GlobalContext`.
 *
 * ## State
 *
 * Notification history is in-memory session state in `GlobalContext`. This
 * feature has no Query data, feature store, URL state, or browser persistence.
 * {@link Notifications} mounts presentation only; it does not own records.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     FEATURE["product feature"] --> MESSAGE["useMessage"]
 *     MESSAGE --> GLOBAL["GlobalContext history"]
 *     MESSAGE --> TOAST["Sonner toast"]
 *     TOAST --> READ["mark record read"]
 *     NAV["app navigation"] --> CENTER["MessageCenter"]
 *     CENTER --> GLOBAL
 * ```
 *
 * Product workflows call {@link useMessage}. The same event is appended to the
 * global history and shown as a toast; dismissing the toast marks its matching
 * record read. {@link MessageCenter} renders recent history and read/clear
 * commands in navigation. {@link Notifications} mounts the process-wide
 * {@link Toaster}.
 *
 * ## Data
 *
 * There is no backend contract. Notification type, message, duration, read
 * state, and clearing are runtime values supplied through `GlobalContext`.
 * The root `index.ts` exports {@link useMessage}, {@link MessageCenter}, and
 * {@link Notifications}; the toaster implementation remains private to app
 * composition.
 *
 * @module
 */
import type MessageCenter from "./components/MessageCenter.tsx";
import type Notifications from "./components/Notifications.tsx";
import type { Toaster } from "./components/Toaster.tsx";
import type { useMessage } from "./hooks/useMessage.ts";

export {};
