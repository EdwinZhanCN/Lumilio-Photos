# Authentication

[AuthProvider](./state/AuthProvider.tsx) owns the verified user session and exposes login, MFA,
completion, and logout commands. HTTP authentication is centralized in
[authMiddleware](@/lib/http-commons/client.ts): it attaches the access token, permits only one
refresh-token rotation at a time, and replays a failed request from a clone
captured before its body was consumed.

Every session exit converges on [resetSession](./state/resetSession.ts). The reset invalidates
late refresh responses, removes tokens, aborts Lumilio streams, cancels and
clears TanStack Query work, clears notifications and agent context, removes
persisted asset filters/search, and clears repository scope preferences.
This boundary runs for explicit logout, failed bootstrap authentication, and
refresh exhaustion so a later user cannot observe the prior user's state.

[registerSessionExpiredHandler](../../lib/http-commons/sessionEvents.ts) connects transport-level refresh
exhaustion back to the provider without making the HTTP client depend on
React or browser navigation.

## Flows

Route entries stay thin and delegate to workflow-owned implementations:

- [useLoginFlow](./flows/sign-in/useLoginFlow.ts) owns identifier-first login, passkey selection,
  password fallback, MFA challenge, and redirect recovery.
- [useRegistrationFlow](./flows/registration/useRegistrationFlow.ts) owns registration plus optional TOTP,
  passkey, and recovery-code onboarding.
- [useMFAFlow](./flows/mfa/useMFAFlow.ts) owns authenticated MFA setup, disable, and recovery-code
  regeneration. Its `mfa` and `action` URL parameters remain authoritative.
- [useBootstrapFlow](./flows/bootstrap/useBootstrapFlow.ts) composes first-admin registration with primary
  repository setup without copying either domain's server state.
- Password-change workflows share [usePasswordConfirmation](./hooks/usePasswordConfirmation.ts) but keep
  their one-use challenge in the session-scoped auth state boundary.

## Capabilities and rules

[ProtectedRoute](./modules/access/ProtectedRoute.tsx), [BootstrapGate](./modules/access/BootstrapGate.tsx), [SetupGate](./modules/access/SetupGate.tsx), and
[PrimaryRepositoryGate](./modules/access/PrimaryRepositoryGate.tsx) are access/setup capabilities used by app
composition, not page components. Browser WebAuthn conversion stays in the
isolated [getPasskeySupport](./modules/webauthn/webauthn.ts) module, while deterministic credential
policy such as [normalizeUsernameInput](./model/credentialPolicy.ts) stays in the React-free model.
