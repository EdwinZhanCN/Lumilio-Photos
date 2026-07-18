/**
 * # Authentication
 *
 * {@link AuthProvider} owns the verified user session and exposes login, MFA,
 * completion, and logout commands. HTTP authentication is centralized in
 * {@link authMiddleware}: it attaches the access token, permits only one
 * refresh-token rotation at a time, and replays a failed request from a clone
 * captured before its body was consumed.
 *
 * Every session exit converges on {@link resetSession}. The reset invalidates
 * late refresh responses, removes tokens, aborts Lumilio streams, cancels and
 * clears TanStack Query work, clears notifications and agent context, removes
 * persisted asset filters/search, and clears repository scope preferences.
 * This boundary runs for explicit logout, failed bootstrap authentication, and
 * refresh exhaustion so a later user cannot observe the prior user's state.
 *
 * {@link registerSessionExpiredHandler} connects transport-level refresh
 * exhaustion back to the provider without making the HTTP client depend on
 * React or browser navigation.
 *
 * @module
 */
import type { AuthProvider } from "./state/AuthProvider.tsx";
import type { resetSession } from "./state/resetSession.ts";
import type { registerSessionExpiredHandler } from "../../lib/http-commons/sessionEvents.ts";
import type { authMiddleware } from "@/lib/http-commons/client.ts";

export {};
