import { useRef, useState } from "react";
import { useI18n } from "@/lib/i18n";

export function useMusicFeedback() {
  const { t } = useI18n();
  const busy = useRef(false);
  const [status, setStatus] = useState<"idle" | "pending" | "success" | "error">("idle");
  const run = async (action: () => Promise<unknown>) => {
    if (busy.current) return;
    busy.current = true;
    setStatus("pending");
    try {
      await action();
      setStatus("success");
    } catch {
      setStatus("error");
    } finally {
      busy.current = false;
    }
  };
  const feedback =
    status === "error" ? (
      <p role="alert" className="px-5 py-2 text-sm text-error">
        {t("music.feedback.error", "Could not save changes. Reload the item and try again.")}
      </p>
    ) : status === "success" ? (
      <p role="status" className="px-5 py-2 text-sm text-success">
        {t("music.feedback.success", "Changes saved.")}
      </p>
    ) : null;
  return { run, feedback, pending: status === "pending" };
}
