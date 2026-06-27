import { Link } from "react-router-dom";
import { ArrowLeft, Pin, Snowflake, Sparkles } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema";
import { useI18n } from "@/lib/i18n";

type AgentPinDTO = components["schemas"]["dto.AgentPinDTO"];

/** Context bar rendered above the gallery when browsing a pinned agent result
 * (`/assets?pin={id}`). Hydrates pin metadata through the same cached query as
 * the gallery, so the extra lookup is free. */
export function PinAssetsHero({ pinId }: { pinId: string }) {
  const { t } = useI18n();
  const query = $api.useQuery(
    "get",
    "/api/v1/agent/pins/{id}",
    { params: { path: { id: pinId } } },
    { retry: false, staleTime: 60_000 },
  );
  const pin = query.data as AgentPinDTO | undefined;
  const isLoading = query.isLoading && !pin;
  const count = pin?.count ?? 0;
  const title =
    pin?.title || t("assets.pin.defaultTitle", { defaultValue: "Agent result" });

  return (
    <div className="px-4 py-4">
      <Link
        to="/lumilio"
        className="btn btn-ghost btn-sm gap-1.5 text-base-content/60"
      >
        <ArrowLeft className="size-4" />
        {t("assets.pin.backToLumilio")}
      </Link>

      <div className="mt-3 flex flex-wrap items-baseline gap-3">
        <span className="badge badge-primary gap-1.5">
          <Pin className="size-3.5" />
          {t("assets.pin.badge")}
        </span>

        {isLoading ? (
          <div className="h-10 w-64 animate-pulse rounded-lg bg-base-300" />
        ) : (
          <h1 className="text-4xl font-black tracking-tight text-primary">
            {title}
          </h1>
        )}

        {count > 0 && (
          <span className="text-xs font-bold uppercase tracking-widest text-base-content/40">
            <span className="mr-1 text-[8px] text-primary">●</span>
            {t("assets.pin.count", { count })}
          </span>
        )}

        {pin?.mode && (
          <span
            className={`badge badge-sm gap-1 ${
              pin.mode === "live" ? "badge-success" : "badge-ghost"
            }`}
          >
            {pin.mode === "live" ? (
              <>
                <Sparkles className="size-3" />
                {t("assets.pin.modeLive")}
              </>
            ) : (
              <>
                <Snowflake className="size-3" />
                {t("assets.pin.modeFrozen")}
              </>
            )}
          </span>
        )}
      </div>

      {pin?.summary && (
        <p className="mt-2 max-w-3xl text-sm leading-relaxed text-base-content/70 line-clamp-2">
          {pin.summary}
        </p>
      )}
    </div>
  );
}
