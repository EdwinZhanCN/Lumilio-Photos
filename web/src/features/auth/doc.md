# Authentication

[AuthProvider](./AuthProvider.tsx) owns the verified user session and exposes login, MFA,
completion, and logout commands. HTTP authentication is centralized in
[authMiddleware](@/lib/http-commons/client.ts): it attaches the access token, permits only one
refresh-token rotation at a time, and replays a failed request from a clone
captured before its body was consumed.

Every session exit converges on [resetSession](./resetSession.ts). The reset invalidates
late refresh responses, removes tokens, aborts Lumilio streams, cancels and
clears TanStack Query work, clears notifications and agent context, removes
persisted asset filters/search, and clears repository scope preferences.
This boundary runs for explicit logout, failed bootstrap authentication, and
refresh exhaustion so a later user cannot observe the prior user's state.

[registerSessionExpiredHandler](../../lib/http-commons/sessionEvents.ts) connects transport-level refresh
exhaustion back to the provider without making the HTTP client depend on
React or browser navigation.
