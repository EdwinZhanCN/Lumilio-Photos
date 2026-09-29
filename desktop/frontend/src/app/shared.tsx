import { useEffect, useRef, useState, type ReactNode } from "react";
import { useTranslation } from "react-i18next";
import {
  type OperationReceipt,
  type OperationSnapshot,
  type ProcessPresentation,
} from "../../bindings/desktop/internal/control/dto/models.js";
import { type AnimatedBadgeStatus } from "@/components/motion/animated-badge";
import { Button, type ButtonState } from "@/components/motion/button";
import { Loader } from "@/components/motion/loader";
import { errorMessage } from "@/lib/desktop/errors";

export const appIconURL = new URL("../../../build/appicon.png", import.meta.url).href;

interface TrackedOperationOptions {
  onSucceeded?: () => void;
  onFailed?: (message: string) => void;
}

/** Desktop service methods acknowledge an operation before their actor work is
 * complete. This hook makes the shared operation registry, rather than the RPC
 * return, authoritative for button success and failure. */
export function useTrackedOperation(
  operations: OperationSnapshot[] | null | undefined,
  options: TrackedOperationOptions = {},
) {
  const [operationID, setOperationID] = useState<string | null>(null);
  const [state, setState] = useState<ButtonState>("idle");
  const [error, setError] = useState<string | null>(null);
  const optionsRef = useRef(options);
  const resetTimer = useRef<number | null>(null);
  optionsRef.current = options;

  useEffect(
    () => () => {
      if (resetTimer.current !== null) window.clearTimeout(resetTimer.current);
    },
    [],
  );

  useEffect(() => {
    if (!operationID) return;
    const operation = operations?.find((item) => item.operationID === operationID);
    if (!operation) return;
    if (operation.state === "accepted" || operation.state === "running") {
      setState("loading");
      return;
    }
    if (operation.state === "succeeded") {
      setOperationID(null);
      setError(null);
      setState("success");
      optionsRef.current.onSucceeded?.();
      resetTimer.current = window.setTimeout(() => {
        setState("idle");
        resetTimer.current = null;
      }, 1200);
      return;
    }
    if (operation.state === "failed" || operation.state === "cancelled") {
      const message = operation.error?.message || `Desktop operation ${operation.state}.`;
      setOperationID(null);
      setError(message);
      setState("error");
      optionsRef.current.onFailed?.(message);
    }
  }, [operationID, operations]);

  const begin = () => {
    if (resetTimer.current !== null) {
      window.clearTimeout(resetTimer.current);
      resetTimer.current = null;
    }
    setOperationID(null);
    setError(null);
    setState("loading");
  };

  const track = (receipt: OperationReceipt) => {
    setOperationID(receipt.operationID);
  };

  const reject = (reason: unknown) => {
    const message = errorMessage(reason);
    setOperationID(null);
    setError(message);
    setState("error");
    optionsRef.current.onFailed?.(message);
  };

  const reset = () => {
    setOperationID(null);
    setError(null);
    setState("idle");
  };

  return { state, error, begin, track, reject, reset };
}

export function ActionNotice({
  component,
  message,
  actionLabel,
  onAction,
}: {
  component: string;
  message: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  const { t } = useTranslation();
  return (
    <div className="action-notice" role="alert">
      <div>
        <strong>{t("actionNotice.needsAttention", { component })}</strong>
        <span>{message}</span>
      </div>
      {actionLabel && onAction ? (
        <Button variant="secondary" size="sm" onClick={onAction}>
          {actionLabel}
        </Button>
      ) : null}
    </div>
  );
}

export function SettingsLoading({ label }: { label: string }) {
  const { t } = useTranslation();
  return (
    <div className="loading-panel">
      <Loader variant="ascii-braille" size={20} label={label} />
      <span>{t("common.loading", { label })}</span>
    </div>
  );
}

export function RowActions({ children }: { children: ReactNode }) {
  return <div className="row-actions">{children}</div>;
}

export function presentationStatus(presentation: ProcessPresentation): AnimatedBadgeStatus {
  if (presentation.color === "green") return "success";
  if (presentation.color === "yellow") return "warning";
  if (presentation.color === "red") return "danger";
  return "neutral";
}
