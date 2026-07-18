# Users

Users is intentionally a small API-only feature. It owns the typed queries
and mutations for listing managed users, updating profiles and access, and
changing the current user's password.

[useUsers](./api/useUsers.ts) keeps the server's user list in TanStack Query and exposes a
normalized empty list while loading. [useUpdateMyProfile](./api/useUsers.ts),
[useAdminUpdateUser](./api/useUsers.ts), and [useResetUserAccess](./api/useUsers.ts) invalidate that
server list after successful changes. [useChangeMyPassword](./api/useUsers.ts) exposes
the current-user password mutation consumed by the Auth password-change
flow.

The feature has no route, workflow UI, client store, or persistence of its
own. Auth and Settings compose its public API, so adding empty structural
directories would obscure rather than clarify ownership.
