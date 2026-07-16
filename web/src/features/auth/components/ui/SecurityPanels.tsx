import React, { useState } from "react";
import {
  ArrowRight,
  Check,
  CircleAlert,
  Download,
  Fingerprint,
  KeyRound,
  ScanFace,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { Btn, CopyButton } from "./Buttons.tsx";
import { AuthQR, OtpInput } from "./Verification.tsx";

const PASSKEY_METHODS: Array<[LucideIcon, string, string]> = [
  [ScanFace, "auth.passkey.affordance.method.face", "Face"],
  [Fingerprint, "auth.passkey.affordance.method.touch", "Touch"],
  [KeyRound, "auth.passkey.affordance.method.security_key", "Security key"],
];

export const PasskeyAffordance: React.FC<{
  created?: boolean;
  headline?: string;
  description?: string;
  activeLabel?: string;
}> = ({ created = false, headline, description, activeLabel }) => {
  const { t } = useI18n();
  const resolvedHeadline =
    headline ??
    t("auth.register.passkeyHeadline", {
      defaultValue: "Sign in with your face or fingerprint",
    });
  const resolvedDescription =
    description ??
    t("auth.register.passkeyDescription", {
      defaultValue:
        "No password to remember. Your passkey stays on this device and can’t be phished.",
    });
  const resolvedActiveLabel =
    activeLabel ??
    t("auth.passkey.affordance.activeLabel", {
      defaultValue: "Passkey active",
    });

  return (
    <>
      <div className="flex flex-col items-center gap-4 rounded-2xl border border-dashed border-base-300 bg-base-200/40 px-5 py-7 text-center">
        <div className="grid h-16 w-16 place-items-center rounded-2xl bg-base-100 shadow-sm">
          {created ? (
            <Check size={30} className="text-success" />
          ) : (
            <Fingerprint size={30} className="text-base-content" />
          )}
        </div>
        {created ? (
          <p className="font-medium text-base-content">{resolvedActiveLabel}</p>
        ) : (
          <div className="space-y-1">
            <p className="font-medium text-base-content">{resolvedHeadline}</p>
            <p className="text-sm leading-relaxed text-base-content/55">{resolvedDescription}</p>
          </div>
        )}
      </div>
      {!created && (
        <div className="grid grid-cols-3 gap-2">
          {PASSKEY_METHODS.map(([Icon, key, fallback]) => (
            <div
              key={key}
              className="flex flex-col items-center gap-1.5 rounded-xl border border-base-200 py-3 text-xs text-base-content/55"
            >
              <Icon size={18} className="text-base-content/45" />{" "}
              {t(key, { defaultValue: fallback })}
            </div>
          ))}
        </div>
      )}
    </>
  );
};

export const RecoveryCodesPanel: React.FC<{
  codes: string[];
  confirmLabel?: string;
  warning?: string;
  checkboxLabel?: string;
  onConfirm: () => void;
  busy?: boolean;
}> = ({ codes, confirmLabel, warning, checkboxLabel, onConfirm, busy }) => {
  const { t } = useI18n();
  const resolvedConfirmLabel =
    confirmLabel ?? t("auth.bootstrap.recovery.continue", { defaultValue: "Continue" });
  const resolvedWarning =
    warning ??
    t("auth.bootstrap.recovery.warning", {
      defaultValue:
        "Each code works once. Store them somewhere safe — they’re the only way back in if you lose your passkey and authenticator.",
    });
  const resolvedCheckboxLabel =
    checkboxLabel ??
    t("auth.bootstrap.recovery.savedConfirm", {
      defaultValue: "I’ve saved my recovery codes somewhere safe",
    });
  const [saved, setSaved] = useState(false);
  const [pulled, setPulled] = useState(false);
  const download = () => {
    setPulled(true);
    try {
      const blob = new Blob(
        [
          `Lumilio Photos — recovery codes\nGenerated ${new Date().toLocaleString()}\n\n${codes.join(
            "\n",
          )}\n`,
        ],
        { type: "text/plain" },
      );
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = "lumilio-recovery-codes.txt";
      a.click();
      URL.revokeObjectURL(a.href);
    } catch {
      /* clipboard/blob unavailable */
    }
  };
  return (
    <>
      <div className="flex items-start gap-2.5 rounded-xl bg-warning/10 px-4 py-3 text-sm text-base-content/75">
        <TriangleAlert size={16} className="mt-0.5 shrink-0 text-warning" />
        <span>{resolvedWarning}</span>
      </div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-2.5 rounded-2xl border border-base-200 bg-base-200/40 p-5 font-mono text-[0.92rem] tabular-nums text-base-content">
        {codes.map((c, i) => (
          <div key={c} className="flex items-center gap-2">
            <span className="w-4 text-right text-xs text-base-content/35">{i + 1}</span>
            <span className="tracking-tight">{c}</span>
          </div>
        ))}
      </div>
      <div className="flex gap-2">
        <button
          type="button"
          onClick={download}
          className="btn btn-outline h-11 min-h-11 flex-1 gap-2 rounded-xl border-base-300 font-medium normal-case text-base-content hover:bg-base-200"
        >
          {pulled ? <Check size={16} /> : <Download size={16} />}{" "}
          {pulled
            ? t("auth.bootstrap.recovery.downloaded", {
                defaultValue: "Downloaded",
              })
            : t("auth.bootstrap.recovery.download", {
                defaultValue: "Download",
              })}
        </button>
        <div className="flex-1">
          <CopyButton
            text={codes.join("\n")}
            label={t("auth.bootstrap.recovery.copyAll", {
              defaultValue: "Copy all",
            })}
            variant="block"
          />
        </div>
      </div>
      <label className="flex cursor-pointer items-center gap-3 rounded-xl border border-base-200 px-4 py-3 transition hover:bg-base-200/40">
        <input
          type="checkbox"
          className="checkbox checkbox-sm"
          checked={saved}
          onChange={(e) => setSaved(e.target.checked)}
        />
        <span className="text-sm text-base-content/80">{resolvedCheckboxLabel}</span>
      </label>
      <Btn variant="primary" icon={ArrowRight} disabled={!saved} loading={busy} onClick={onConfirm}>
        {resolvedConfirmLabel}
      </Btn>
    </>
  );
};

export const TotpSetupPanel: React.FC<{
  otpauthUri: string;
  secret: string;
  code: string;
  onCodeChange: (value: string) => void;
  onVerify: () => void;
  invalid?: boolean;
  busy?: boolean;
  errorMessage?: string | null;
  verifyLabel?: string;
  steps?: { open: string; openSub: string; scan: string; enter: string };
}> = ({
  otpauthUri,
  secret,
  code,
  onCodeChange,
  onVerify,
  invalid,
  busy,
  errorMessage,
  verifyLabel,
  steps,
}) => {
  const { t } = useI18n();
  const [reveal, setReveal] = useState(false);
  const copy = {
    open:
      steps?.open ??
      t("auth.bootstrap.totp.setup.openApp", {
        defaultValue: "Open your authenticator app",
      }),
    openSub:
      steps?.openSub ??
      t("auth.bootstrap.totp.setup.openAppSub", {
        defaultValue: "Google Authenticator, 1Password, Authy, or similar.",
      }),
    scan:
      steps?.scan ??
      t("auth.bootstrap.totp.setup.scanQr", {
        defaultValue: "Scan this QR code",
      }),
    enter:
      steps?.enter ??
      t("auth.bootstrap.totp.setup.enterCode", {
        defaultValue: "Enter the 6-digit code",
      }),
  };
  const resolvedVerifyLabel =
    verifyLabel ??
    t("auth.bootstrap.totp.setup.verifyButton", {
      defaultValue: "Verify & enable",
    });
  const stepNum = (n: number) => (
    <span className="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-base-200 text-xs font-semibold text-base-content/70">
      {n}
    </span>
  );
  return (
    <>
      <ol className="space-y-3 text-sm">
        <li className="flex gap-3">
          {stepNum(1)}
          <div className="pt-0.5">
            <p className="font-medium text-base-content">{copy.open}</p>
            <p className="text-base-content/55">{copy.openSub}</p>
          </div>
        </li>
        <li className="flex gap-3">
          {stepNum(2)}
          <div className="w-full pt-0.5">
            <p className="font-medium text-base-content">{copy.scan}</p>
            <div className="mt-3 flex flex-col items-center gap-2.5">
              <AuthQR value={otpauthUri} />
              <button
                type="button"
                onClick={() => setReveal((r) => !r)}
                className="text-xs font-medium text-base-content/45 underline-offset-2 hover:underline"
              >
                {reveal
                  ? t("auth.bootstrap.totp.setup.hideKey", {
                      defaultValue: "Hide setup key",
                    })
                  : t("auth.bootstrap.totp.setup.showKey", {
                      defaultValue: "Can’t scan? Enter key manually",
                    })}
              </button>
              {reveal && (
                <div className="flex items-center gap-1 rounded-lg border border-base-200 bg-base-200/50 py-1 pl-3 pr-1">
                  <code className="text-sm font-medium tracking-wider text-base-content">
                    {secret}
                  </code>
                  <CopyButton text={secret.replace(/ /g, "")} />
                </div>
              )}
            </div>
          </div>
        </li>
        <li className="flex gap-3">
          {stepNum(3)}
          <div className="w-full pt-0.5">
            <p className="font-medium text-base-content">{copy.enter}</p>
            <div className="mt-3">
              <OtpInput value={code} onChange={onCodeChange} invalid={invalid} autoFocus={false} />
              {invalid && errorMessage && (
                <p className="mt-2 flex items-center gap-1 text-xs font-medium text-error">
                  <CircleAlert size={13} /> {errorMessage}
                </p>
              )}
            </div>
          </div>
        </li>
      </ol>
      <Btn variant="primary" loading={busy} onClick={onVerify} disabled={code.length < 6}>
        {resolvedVerifyLabel}
      </Btn>
    </>
  );
};
