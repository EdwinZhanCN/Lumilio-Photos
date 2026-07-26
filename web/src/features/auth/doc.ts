/**
 * # Authentication
 *
 * {@link AuthProvider} owns the verified user session and exposes login, MFA,
 * completion, and logout commands. HTTP authentication is centralized in
 * {@link authMiddleware}: it attaches the short-lived access token, supplies
 * the session-bound CSRF proof to unsafe requests, and replays a failed request
 * from a clone captured before its body was consumed.
 *
 * The long-lived refresh credential exists only in a host-only `HttpOnly`
 * cookie scoped to `/api/v1/auth`; browser JavaScript never receives or stores
 * it. {@link refreshBrowserSession} recovers a non-secret CSRF proof from the
 * cookie session, rotates the cookie and access token under a cross-tab Web
 * Lock, and is also the bootstrap session probe. {@link logoutBrowserSession}
 * uses the same lock so logout cannot race a refresh in another tab.
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
 * ## Flows
 *
 * Route entries stay thin and delegate to workflow-owned implementations:
 *
 * - {@link useLoginFlow} owns identifier-first login, passkey selection,
 *   password fallback, MFA challenge, and redirect recovery.
 * - {@link useRegistrationFlow} owns registration plus optional TOTP,
 *   passkey, and recovery-code onboarding.
 * - {@link useMFAFlow} owns authenticated MFA setup, disable, and recovery-code
 *   regeneration. Its `mfa` and `action` URL parameters remain authoritative.
 * - {@link useBootstrapFlow} composes first-admin registration with primary
 *   repository setup without copying either domain's server state.
 * - Password-change workflows share {@link usePasswordConfirmation} but keep
 *   their one-use challenge in the session-scoped auth state boundary.
 *
 * ## Capabilities and rules
 *
 * {@link ProtectedRoute}, {@link BootstrapGate}, and
 * {@link PrimaryRepositoryGate} are access/setup capabilities used by app
 * composition, not page components. Browser WebAuthn conversion stays in the
 * isolated {@link getPasskeySupport} module, while deterministic credential
 * policy such as {@link normalizeUsernameInput} stays in the React-free model.
 *
 * @module
 */
import type { useBootstrapFlow } from "./flows/bootstrap/useBootstrapFlow.ts";
import type { useMFAFlow } from "./flows/mfa/useMFAFlow.ts";
import type { useRegistrationFlow } from "./flows/registration/useRegistrationFlow.ts";
import type { useLoginFlow } from "./flows/sign-in/useLoginFlow.ts";
import type { usePasswordConfirmation } from "./hooks/usePasswordConfirmation.ts";
import type { normalizeUsernameInput } from "./model/credentialPolicy.ts";
import type BootstrapGate from "./modules/access/BootstrapGate.tsx";
import type PrimaryRepositoryGate from "./modules/access/PrimaryRepositoryGate.tsx";
import type ProtectedRoute from "./modules/access/ProtectedRoute.tsx";
import type { getPasskeySupport } from "./modules/webauthn/webauthn.ts";
import type { AuthProvider } from "./state/AuthProvider.tsx";
import type { resetSession } from "./state/resetSession.ts";
import type { registerSessionExpiredHandler } from "../../lib/http-commons/sessionEvents.ts";
import type {
  authMiddleware,
  logoutBrowserSession,
  refreshBrowserSession,
} from "@/lib/http-commons/client.ts";

export {};
