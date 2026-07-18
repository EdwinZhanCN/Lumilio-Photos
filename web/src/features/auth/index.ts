export { AuthProvider } from "./state/AuthProvider";
export { useAuth } from "./state/useAuth";
export { default as ProtectedRoute } from "./modules/access/ProtectedRoute";
export { default as BootstrapGate } from "./modules/access/BootstrapGate";
export { default as PrimaryRepositoryGate } from "./modules/access/PrimaryRepositoryGate";
export { default as SetupGate } from "./modules/access/SetupGate";
export { useMFAStatus } from "./api/useMFA";
export {
  useBeginPasskeyEnrollment,
  useDeletePasskey,
  usePasskeys,
  useVerifyPasskeyEnrollment,
} from "./api/usePasskeys";
export {
  DISPLAY_NAME_HINT,
  DISPLAY_NAME_MAX_LENGTH,
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
} from "./model/credentialPolicy";
export { createPasskeyCredential, getPasskeySupport } from "./modules/webauthn/webauthn";
