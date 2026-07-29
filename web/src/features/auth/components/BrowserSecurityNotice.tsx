import { TriangleAlert } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useBrowserCapabilities } from "../api/useBrowserCapabilities.ts";

export function BrowserSecurityNotice({ className = "" }: { className?: string }) {
  const { t } = useI18n();
  const capabilities = useBrowserCapabilities();

  if (!capabilities.data?.insecure_transport) {
    return null;
  }

  return (
    <div role="alert" className={`alert alert-warning alert-soft ${className}`}>
      <TriangleAlert className="size-5 shrink-0" aria-hidden="true" />
      <div className="text-sm">
        <p className="font-semibold">
          {t("auth.browserSecurity.insecureTitle", {
            defaultValue: "This network connection is not encrypted.",
          })}
        </p>
        <p className="mt-1">
          {t("auth.browserSecurity.insecureBody", {
            defaultValue:
              "Passwords, authenticator codes, session cookies, and media can be read on this network. Passkeys work on localhost or over HTTPS; remote HTTP devices must use password and TOTP.",
          })}
        </p>
      </div>
    </div>
  );
}
