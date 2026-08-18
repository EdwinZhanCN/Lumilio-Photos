/**
 * # Authentication
 *
 * Authentication owns the verified browser session, public sign-in and
 * registration flows, MFA and password challenges, first-run bootstrap, and
 * application access gates. Profile editing and ordinary user administration
 * remain in Users and Settings.
 *
 * ## State
 *
 * {@link AuthProvider} owns the session reducer and the verified user. Browser
 * JavaScript stores only the short-lived access token and session-bound CSRF
 * proof; the refresh credential remains a host-only `HttpOnly` cookie.
 * {@link refreshBrowserSession} rotates the cookie and access token under a
 * cross-tab Web Lock, and {@link logoutBrowserSession} uses the same lock.
 *
 * Password-change and MFA challenges are one-use, session-scoped values; they
 * are not written to URL history or browser persistence. Every session exit
 * converges on {@link resetSession}, which removes credentials, resets feature
 * and global runtime state, clears user-scoped preferences, cancels Query work,
 * and clears the Query cache before another user can authenticate.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     PUBLIC["Login / Register"] --> AUTH["AuthProvider"]
 *     AUTH --> MFA["MFA flow"]
 *     AUTH --> PASSWORD["Password-change flow"]
 *     SETUP["Bootstrap flow"] --> REGISTER["first admin"]
 *     SETUP --> REPOSITORY["primary repository"]
 *     AUTH --> GATES["ProtectedRoute / setup gates"]
 *     GATES --> APP["authenticated app"]
 * ```
 *
 * {@link useLoginFlow} coordinates identifier-first login, passkey selection,
 * password fallback, MFA challenge, and redirect recovery.
 * {@link useRegistrationFlow} handles registration plus optional TOTP,
 * passkey, and recovery-code onboarding. {@link useMFAFlow} owns authenticated
 * MFA management, with its `mfa` and `action` URL parameters authoritative.
 * {@link useBootstrapFlow} composes first-admin registration with repository
 * creation without copying either domain's server state.
 *
 * {@link ProtectedRoute}, {@link BootstrapGate}, and
 * {@link PrimaryRepositoryGate} are composition capabilities, not page
 * implementations. Route files only re-export their owning flows.
 *
 * ## Data
 *
 * {@link authMiddleware} attaches the access token and CSRF proof, then retries
 * one failed request from a clone captured before body consumption.
 * {@link registerSessionExpiredHandler} reports transport refresh exhaustion
 * back to React without coupling the HTTP client to routing.
 * Auth and bootstrap flows preserve generated Problem objects until their
 * presentation boundary calls {@link localizeAPIProblem} with the current
 * language and an operation-specific fallback. Session recovery branches on
 * HTTP status or exact Problem type, never on localized or server-authored copy.
 *
 * Reusable auth queries and mutations live in `api/`; browser WebAuthn
 * conversion stays in {@link getPasskeySupport}, and deterministic account
 * policy stays in the React-free model through
 * {@link normalizeUsernameInput}. {@link useBrowserCapabilities} reads the
 * server-resolved origin/security contract used by public flows and
 * {@link BrowserSecurityNotice}. The root `index.ts` is the only runtime
 * cross-feature entry.
 *
 * @module
 */
import type { useBrowserCapabilities } from "./api/useBrowserCapabilities.ts";
import type { BrowserSecurityNotice } from "./components/BrowserSecurityNotice.tsx";
import type { useBootstrapFlow } from "./flows/bootstrap/useBootstrapFlow.ts";
import type { useMFAFlow } from "./flows/mfa/useMFAFlow.ts";
import type { useRegistrationFlow } from "./flows/registration/useRegistrationFlow.ts";
import type { useLoginFlow } from "./flows/sign-in/useLoginFlow.ts";
import type { normalizeUsernameInput } from "./model/credentialPolicy.ts";
import type BootstrapGate from "./modules/access/BootstrapGate.tsx";
import type PrimaryRepositoryGate from "./modules/access/PrimaryRepositoryGate.tsx";
import type ProtectedRoute from "./modules/access/ProtectedRoute.tsx";
import type { getPasskeySupport } from "./modules/webauthn/webauthn.ts";
import type { AuthProvider } from "./state/AuthProvider.tsx";
import type { resetSession } from "./state/resetSession.ts";
import type {
  authMiddleware,
  logoutBrowserSession,
  refreshBrowserSession,
} from "../../lib/http-commons/client.ts";
import type { registerSessionExpiredHandler } from "../../lib/http-commons/sessionEvents.ts";
import type { localizeAPIProblem } from "../../lib/http-commons/problem.ts";

export {};
