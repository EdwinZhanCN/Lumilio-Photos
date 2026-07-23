import React, { useEffect, useRef, useState } from "react";
import QRCode from "qrcode";
import { useI18n } from "@/lib/i18n";
import { cx } from "./classNames.ts";

export const OtpInput: React.FC<{
  length?: number;
  value: string;
  onChange: (value: string) => void;
  onComplete?: (value: string) => void;
  autoFocus?: boolean;
  invalid?: boolean;
}> = ({ length = 6, value, onChange, onComplete, autoFocus = true, invalid }) => {
  const refs = useRef<Array<HTMLInputElement | null>>([]);
  const chars = Array.from({ length }, (_, i) => value[i] ?? "");

  useEffect(() => {
    if (autoFocus) refs.current[0]?.focus();
  }, [autoFocus]);

  const setAt = (i: number, ch: string) => {
    const next = (value.slice(0, i) + ch + value.slice(i + 1)).slice(0, length);
    onChange(next);
    return next;
  };

  const handle = (i: number, e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value.replace(/\D/g, "");
    if (!v) return;
    const next = setAt(i, v[v.length - 1]);
    if (i < length - 1) refs.current[i + 1]?.focus();
    if (next.length === length) onComplete?.(next);
  };

  const key = (i: number, e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Backspace") {
      e.preventDefault();
      if (chars[i]) {
        setAt(i, "");
      } else if (i > 0) {
        refs.current[i - 1]?.focus();
        setAt(i - 1, "");
      }
    }
    if (e.key === "ArrowLeft" && i > 0) refs.current[i - 1]?.focus();
    if (e.key === "ArrowRight" && i < length - 1) refs.current[i + 1]?.focus();
  };

  const paste = (e: React.ClipboardEvent<HTMLDivElement>) => {
    e.preventDefault();
    const txt = (e.clipboardData.getData("text") || "").replace(/\D/g, "").slice(0, length);
    if (!txt) return;
    onChange(txt);
    refs.current[Math.min(txt.length, length - 1)]?.focus();
    if (txt.length === length) onComplete?.(txt);
  };

  return (
    <div className="flex justify-between gap-2" onPaste={paste}>
      {chars.map((c, i) => (
        <input
          // eslint-disable-next-line react/no-array-index-key
          key={i}
          ref={(el) => {
            refs.current[i] = el;
          }}
          inputMode="numeric"
          autoComplete={i === 0 ? "one-time-code" : "off"}
          maxLength={1}
          value={c}
          onChange={(e) => handle(i, e)}
          onKeyDown={(e) => key(i, e)}
          className={cx(
            "h-14 w-full rounded-xl border bg-base-100 text-center text-2xl font-semibold tabular-nums text-base-content outline-none transition",
            invalid
              ? "border-error"
              : "border-base-300 focus:border-primary focus:ring-2 focus:ring-primary/15",
          )}
        />
      ))}
    </div>
  );
};

export const AuthQR: React.FC<{ value: string; size?: number }> = ({ value, size = 168 }) => {
  const { t } = useI18n();
  const [dataUrl, setDataUrl] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    if (!value) {
      setDataUrl(null);
      return;
    }
    QRCode.toDataURL(value, {
      errorCorrectionLevel: "M",
      margin: 1,
      width: size * 2,
      color: { dark: "#111111", light: "#ffffff" },
    })
      .then((url) => {
        if (!cancelled) setDataUrl(url);
      })
      .catch(() => {
        if (!cancelled) setDataUrl(null);
      });
    return () => {
      cancelled = true;
    };
  }, [value, size]);

  return (
    <div className="inline-block rounded-2xl border border-base-200 bg-white p-3.5 shadow-sm">
      {dataUrl ? (
        <img
          src={dataUrl}
          alt={t("auth.mfa.qrAlt", { defaultValue: "Authenticator QR code" })}
          width={size}
          height={size}
        />
      ) : (
        <div
          className="grid place-items-center text-base-content/30"
          style={{ width: size, height: size }}
        >
          <span className="loading loading-spinner" />
        </div>
      )}
    </div>
  );
};
