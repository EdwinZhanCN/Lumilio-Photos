export * from "./AuthProvider";
export * from "./hooks/useAuth";
export * from "./auth.type.ts";
export { default as ProtectedRoute } from "./components/ProtectedRoute";
export { default as BootstrapGate } from "./components/BootstrapGate";
export { default as PrimaryRepositoryGate } from "./components/PrimaryRepositoryGate";
export { default as SetupGate } from "./components/SetupGate";
export { useMFAStatus } from "./hooks/useMFA";
export {
  useBeginPasskeyEnrollment,
  useDeletePasskey,
  usePasskeys,
  useVerifyPasskeyEnrollment,
} from "./hooks/usePasskeys";
export {
  DISPLAY_NAME_HINT,
  DISPLAY_NAME_MAX_LENGTH,
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
} from "./lib/credentialPolicy";
export { createPasskeyCredential, getPasskeySupport } from "./lib/webauthn";
