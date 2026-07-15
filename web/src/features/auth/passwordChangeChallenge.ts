export type RequiredPasswordChangeChallenge = {
  passwordChangeToken: string;
  username?: string;
  redirectTo?: string;
};

// This challenge deliberately lives only in the current JavaScript runtime.
// It is never written to history.state, sessionStorage, or localStorage, so a
// page refresh discards the one-purpose credential and requires another login.
let pendingChallenge: RequiredPasswordChangeChallenge | null = null;

export function storeRequiredPasswordChangeChallenge(
  challenge: RequiredPasswordChangeChallenge,
): void {
  pendingChallenge = { ...challenge };
}

export function takeRequiredPasswordChangeChallenge(): RequiredPasswordChangeChallenge | null {
  const challenge = pendingChallenge;
  pendingChallenge = null;
  return challenge;
}

export function clearRequiredPasswordChangeChallenge(): void {
  pendingChallenge = null;
}
