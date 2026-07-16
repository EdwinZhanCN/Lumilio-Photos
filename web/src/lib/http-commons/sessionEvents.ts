type SessionExpiredHandler = () => void | Promise<void>;

let sessionExpiredHandler: SessionExpiredHandler | null = null;
let expirationNotified = false;

/** Connects transport-level refresh exhaustion to the application session owner. */
export function registerSessionExpiredHandler(handler: SessionExpiredHandler): () => void {
  sessionExpiredHandler = handler;
  return () => {
    if (sessionExpiredHandler === handler) sessionExpiredHandler = null;
  };
}

/** Notifies the registered session owner at most once until new tokens are saved. */
export function notifySessionExpired(): void {
  if (expirationNotified) return;
  expirationNotified = true;
  void sessionExpiredHandler?.();
}

/** Arms session-expiration notification for the newly authenticated session. */
export function resetSessionExpiredNotification(): void {
  expirationNotified = false;
}
