# Notifications

Notifications is the narrow feature facade for the application's global
notification runtime. Durable in-session notification records remain owned
by `GlobalContext`; this feature does not mirror them in a feature store.

## Surfaces

[useMessage](./hooks/useMessage.ts) is the public command used by product workflows. A call
adds one record to the global message center and presents the same event as
a Sonner toast. Dismissing the toast marks the matching record as read.

[MessageCenter](./components/MessageCenter.tsx) renders the recent notification history in the app
navigation and delegates read/clear commands to `GlobalContext`.
[Notifications](./components/Notifications.tsx) mounts the process-wide [Toaster](./components/Toaster.tsx); it contains
no notification data and only selects theme-aware toast presentation.

## Decisions

The feature intentionally keeps `components/` and `hooks/` rather than
inventing a workflow or state directory. Its UI is reused by app
composition, while notification state is a genuinely cross-cutting runtime
concern already owned by `GlobalContext`.
