import type { PropsWithChildren } from "react";
import { ErrorBoundary, type FallbackProps } from "react-error-boundary";
import { Home, RefreshCw, TriangleAlert } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";

export function RootErrorFallback({ error, resetErrorBoundary }: FallbackProps): React.ReactNode {
  const { t } = useI18n();
  const message = error instanceof Error ? error.message : String(error);

  return (
    <main className="hero min-h-screen bg-base-200 px-4" aria-labelledby="root-error-title">
      <div className="hero-content max-w-2xl flex-col text-center">
        <TriangleAlert className="size-14 text-error" aria-hidden="true" />
        <div>
          <p className="mb-2 font-mono text-sm font-semibold uppercase tracking-widest text-error">
            {t("rootError.label", "Application error")}
          </p>
          <h1 id="root-error-title" className="text-3xl font-bold sm:text-4xl">
            {t("rootError.title", "Lumilio could not continue")}
          </h1>
          <p className="mt-4 text-base-content/70">
            {t(
              "rootError.description",
              "Reload the application to recover. Your original photos and local library are not changed by this error.",
            )}
          </p>
          {message && (
            <p className="mt-4 break-words rounded-box bg-base-300 px-4 py-3 font-mono text-sm text-base-content/70">
              {message}
            </p>
          )}
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <button type="button" className="btn btn-primary" onClick={resetErrorBoundary}>
              <RefreshCw className="size-4" aria-hidden="true" />
              {t("rootError.reload", "Reload application")}
            </button>
            <a className="btn btn-ghost" href="/">
              <Home className="size-4" aria-hidden="true" />
              {t("rootError.home", "Return home")}
            </a>
          </div>
        </div>
      </div>
    </main>
  );
}

export default function RootErrorBoundary({ children }: PropsWithChildren): React.ReactNode {
  return (
    <ErrorBoundary
      FallbackComponent={RootErrorFallback}
      onReset={() => window.location.reload()}
      onError={(error, info) => console.error("[app] unhandled render error", error, info)}
    >
      {children}
    </ErrorBoundary>
  );
}
