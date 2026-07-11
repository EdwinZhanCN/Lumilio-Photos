import { ArrowLeft, Home, SearchX } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";

export default function NotFound(): React.ReactNode {
  const { t } = useI18n();
  const navigate = useNavigate();

  return (
    <main className="hero min-h-screen bg-base-200 px-4" aria-labelledby="not-found-title">
      <div className="hero-content max-w-2xl flex-col text-center">
        <SearchX className="size-14 text-primary" aria-hidden="true" />
        <div>
          <p className="mb-2 font-mono text-sm font-semibold uppercase tracking-widest text-primary">
            {t("notFound.label", "Error 404")}
          </p>
          <h1 id="not-found-title" className="text-3xl font-bold sm:text-4xl">
            {t("notFound.title", "This page does not exist")}
          </h1>
          <p className="mt-4 text-base-content/70">
            {t(
              "notFound.description",
              "The address may be outdated or mistyped. Your library and photos are still safe.",
            )}
          </p>
          <div className="mt-6 flex flex-wrap justify-center gap-3">
            <Link className="btn btn-primary" to="/">
              <Home className="size-4" aria-hidden="true" />
              {t("notFound.home", "Go to library")}
            </Link>
            <button type="button" className="btn btn-ghost" onClick={() => navigate(-1)}>
              <ArrowLeft className="size-4" aria-hidden="true" />
              {t("notFound.back", "Go back")}
            </button>
          </div>
        </div>
      </div>
    </main>
  );
}
