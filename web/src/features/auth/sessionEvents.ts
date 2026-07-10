type SessionExpiredHandler = () => void | Promise<void>;

let sessionExpiredHandler: SessionExpiredHandler | null = null;
let expirationNotified = false;

export function registerSessionExpiredHandler(handler: SessionExpiredHandler): () => void {
  sessionExpiredHandler = handler;
  return () => {
    if (sessionExpiredHandler === handler) sessionExpiredHandler = null;
  };
}

export function notifySessionExpired(): void {
  if (expirationNotified) return;
  expirationNotified = true;
  void sessionExpiredHandler?.();
}

export function resetSessionExpiredNotification(): void {
  expirationNotified = false;
}
