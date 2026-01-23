import React from "react";
import { FallbackProps } from "react-error-boundary";
import { Link } from "react-router-dom";
import {
  AlertTriangle,
  Home,
  RefreshCw,
  Bug,
  ExternalLink,
  Copy,
} from "lucide-react";

type ErrorFallBackProps = {
  code: string | number;
  title: string;
  message?: string;
  error?: any;
  resetErrorBoundary?: (...args: any[]) => void;
};

export default function ErrorFallBack({
  code,
  title,
  message,
  error,
  resetErrorBoundary,
}: ErrorFallBackProps & FallbackProps): React.ReactElement {
  const [copied, setCopied] = React.useState(false);

  const details = React.useMemo(() => {
    const href =
      typeof window !== "undefined" ? window.location.href : "(unknown)";
    const ua =
      typeof navigator !== "undefined" ? navigator.userAgent : "(unknown)";
    const time = new Date().toISOString();

    return [
      "Lumilio-Photos Web - Error Report",
      "",
      `Code: ${code ?? "(n/a)"}`,
      `Title: ${title ?? "(n/a)"}`,
      `Message: ${message || error?.message || String(error) || "(none)"}`,
      `URL: ${href}`,
      `UserAgent: ${ua}`,
      `Time: ${time}`,
      "",
      "Stack:",
      error?.stack || "(none)",
    ].join("\n");
  }, [code, title, message, error]);

  const issueUrl = React.useMemo(() => {
    const base = "https://github.com/EdwinZhanCN/Lumilio-Photos/issues/new";
    const titleParam =
      `[Bug] ${code ?? ""} ${title ?? "Unhandled error"}`.trim();
    const params = new URLSearchParams({
      title: titleParam,
      body: details,
      labels: "bug",
    });
    return `${base}?${params.toString()}`;
  }, [code, title, details]);

  const onCopyDetails = async () => {
    try {
      await navigator.clipboard.writeText(details);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  };

  const displayMessage =
    message || error?.message || "Something went wrong. Please try again.";

  return (
    <main
      className="min-h-screen bg-base-200 flex items-center justify-center p-4"
      role="main"
      aria-labelledby="error-title"
    >
      <div className="card w-full max-w-3xl bg-base-100 shadow-xl max-h-[85vh] overflow-hidden">
        <div className="card-body items-center text-center overflow-auto">
          <div className="flex items-center gap-2">
            <AlertTriangle className="size-6 text-error" aria-hidden="true" />
            <span className="sr-only">Error code</span>
            <div className="badge badge-error badge-lg font-mono">{code}</div>
          </div>

          <h1
            id="error-title"
            className="card-title text-3xl sm:text-4xl mt-2 text-balance"
          >
            {title}
          </h1>

          <p
            className="text-base-content/70 mt-2 text-pretty"
            aria-live="polite"
          >
            {displayMessage}
          </p>

          <div className="w-full mt-4">
            <div className="collapse collapse-arrow border border-base-300 bg-base-100">
              <input type="checkbox" aria-label="Toggle error details" />
              <div className="collapse-title text-left font-medium flex items-center justify-between gap-2">
                <span>View technical details</span>
              </div>
              <div className="collapse-content max-h-64 md:max-h-80 overflow-auto">
                <div className="mockup-code w-full">
                  <pre>
                    <code>{details}</code>
                  </pre>
                </div>
              </div>
            </div>
          </div>

          <div className="card-actions justify-center mt-4 flex-wrap gap-2">
            <Link to="/" className="btn btn-primary">
              <Home className="size-4" aria-hidden="true" />
              <span className="ml-1">Go home</span>
            </Link>

            {resetErrorBoundary && (
              <button
                type="button"
                onClick={resetErrorBoundary}
                className="btn btn-secondary"
              >
                <RefreshCw className="size-4" aria-hidden="true" />
                <span className="ml-1">Try again</span>
              </button>
            )}

            <a
              href={issueUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="btn btn-outline"
            >
              <Bug className="size-4" aria-hidden="true" />
              <span className="ml-1">Report issue</span>
              <ExternalLink className="size-4 ml-1" aria-hidden="true" />
            </a>

            <button
              type="button"
              onClick={onCopyDetails}
              className="btn btn-ghost"
            >
              <Copy className="size-4" aria-hidden="true" />
              <span className="ml-1">
                {copied ? "Copied!" : "Copy details"}
              </span>
            </button>
          </div>
        </div>
      </div>

      {copied && (
        <div className="toast toast-end">
          <div className="alert alert-success">
            <span>Copied error details to clipboard.</span>
          </div>
        </div>
      )}
    </main>
  );
}
