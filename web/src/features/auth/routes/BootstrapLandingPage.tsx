import React from "react";
import { Link } from "react-router-dom";
import { UserPlus } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";

const BootstrapLandingPage: React.FC = () => {
  const { t } = useI18n();

  return (
    <div className="relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-base-200 px-4 py-12">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute -right-48 -top-48 h-96 w-96 rounded-full bg-primary/8 blur-3xl" />
        <div className="absolute -bottom-48 -left-48 h-96 w-96 rounded-full bg-secondary/8 blur-3xl" />
      </div>

      <div className="relative w-full max-w-md">
        <div className="card bg-base-100 shadow-2xl ring-1 ring-base-content/5">
          <div className="card-body gap-6 p-8 sm:p-10">
            <div className="flex flex-col items-center gap-5 text-center">
              <div className="inline-flex items-center gap-4 text-3xl font-bold tracking-tight text-base-content sm:text-4xl">
                <img
                  src="/logo.png"
                  alt={t("auth.common.logoAlt", {
                    appName: t("app.name"),
                  })}
                  className="size-10 bg-contain object-contain sm:size-12"
                />
                <span>{t("app.name")}</span>
              </div>

              <div className="space-y-1.5">
                <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">
                  {t("auth.bootstrap.landing.title", {
                    defaultValue: "Set up your Admin account",
                  })}
                </h1>
              </div>
            </div>

            <div className="rounded-xl border border-warning/40 bg-warning/10 p-4">
              <p className="text-sm font-semibold text-warning">
                {t("auth.bootstrap.landing.promptTitle", {
                  defaultValue: "Initial setup required",
                })}
              </p>
              <p className="mt-1.5 text-xs text-base-content/80">
                {t("auth.bootstrap.landing.promptBody", {
                  defaultValue:
                    "The first registration becomes Admin and will guide you through passkey or authenticator setup.",
                })}
              </p>
            </div>

            <Link
              to="/bootstrap/register"
              className="btn btn-primary w-full gap-2"
            >
              <UserPlus className="h-4 w-4" />
              {t("auth.bootstrap.landing.cta", {
                defaultValue: "Create the first Admin account",
              })}
            </Link>
          </div>
        </div>
      </div>
    </div>
  );
};

export default BootstrapLandingPage;
