import { Camera, Heart, MapPin, Star } from "lucide-react";
import type { ReactNode } from "react";
import type { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n.tsx";
import type { AgentRefDTO } from "../types";
import { inferFacetGranularity } from "./timeBucketLabels";
import type { WidgetProps } from "./types";
import { useWidgetAssetsPreview } from "./useWidgetAssets";
import { useWidgetMetadata } from "./useWidgetMetadata";
import { WidgetAssetThumbnail } from "./WidgetAssetThumbnail";

type Facets = NonNullable<AgentRefDTO["facets"]>;
type NameCount = NonNullable<Facets["top_places"]>[number];

function compact(value: number, locale?: string): string {
  return new Intl.NumberFormat(locale, {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

/** The facet dashboard reads a collection like an album: a photo cover with
 * headline stats, then who/where/what — people as faces, places ranked,
 * camera as a chip. No gray bar charts (that's the timeline's job). */
export function FacetDashboardWidget({ source, variant, count, title }: WidgetProps) {
  const { t, i18n } = useI18n();
  const locale = i18n.language;
  const { facets, isLoading, isError } = useWidgetMetadata(source);
  const cover = useWidgetAssetsPreview(source, 4);

  if (isLoading) return <DashboardSkeleton variant={variant} title={title} />;
  if (isError || !facets) {
    return (
      <DashboardShell variant={variant} title={title}>
        <div className="flex h-24 items-center justify-center text-center text-xs text-base-content/50">
          {t("lumilio.widgets.metadataUnavailable", "This widget's summary is unavailable.")}
        </div>
      </DashboardShell>
    );
  }

  const ratingDist = facets.rating_dist ?? [];
  const ratedCount = ratingDist.reduce((sum, value, index) => sum + (index > 0 ? value : 0), 0);
  const types = Object.entries(facets.types ?? {}).filter(([, value]) => value > 0);

  return (
    <DashboardShell variant={variant} title={title}>
      <CoverHero
        assets={cover.assets}
        loading={cover.isLoading}
        source={source}
        count={count}
        range={formatDateRange(facets, locale)}
        liked={facets.liked_count ?? 0}
        rated={ratedCount}
        locale={locale}
      />

      <FacetSection icon={<MapPin size={14} />} label={t("lumilio.widgets.facets.places", "Places")}>
        <RankedChips values={facets.top_places} />
      </FacetSection>

      <FacetSection
        icon={<Heart size={14} />}
        label={t("lumilio.widgets.facets.people", "People")}
        hidden={!facets.top_people?.length}
      >
        <AvatarStack values={facets.top_people} />
      </FacetSection>

      <div className="flex flex-wrap items-start gap-x-6 gap-y-3">
        <FacetSection
          icon={<Camera size={14} />}
          label={t("lumilio.widgets.facets.cameras", "Cameras")}
          hidden={!facets.cameras?.length}
        >
          <div className="flex flex-wrap gap-1.5">
            {(facets.cameras ?? []).slice(0, 2).map((cam) => (
              <span
                key={cam.name}
                className="inline-flex items-center gap-1.5 rounded-lg border border-base-300 bg-base-200/40 px-2.5 py-1.5 text-xs"
              >
                <Camera size={13} className="text-base-content/50" />
                <span className="font-medium text-base-content">{cam.name}</span>
                <span className="text-base-content/45">{cam.count}</span>
              </span>
            ))}
          </div>
        </FacetSection>

        {types.length > 0 && (
          <FacetSection label={t("lumilio.widgets.facets.types", "Types")}>
            <div className="flex flex-wrap gap-1.5">
              {types.map(([name, value]) => (
                <span key={name} className="badge badge-outline badge-sm">
                  {name.toLowerCase()} {value}
                </span>
              ))}
            </div>
          </FacetSection>
        )}
      </div>
    </DashboardShell>
  );
}

function DashboardShell({
  variant,
  title,
  children,
}: {
  variant: WidgetProps["variant"];
  title?: string;
  children: ReactNode;
}) {
  return (
    <div
      className={
        variant === "board"
          ? "h-full space-y-4 overflow-y-auto p-3"
          : "my-3 max-w-md space-y-4 rounded-xl border border-base-300 bg-base-100 p-3"
      }
    >
      {variant === "inline" && title && (
        <div className="text-sm font-medium text-base-content/80">{title}</div>
      )}
      {children}
    </div>
  );
}

function DashboardSkeleton({ variant, title }: { variant: WidgetProps["variant"]; title?: string }) {
  return (
    <DashboardShell variant={variant} title={title}>
      <div className="skeleton h-36 rounded-2xl" />
      <div className="skeleton h-8 w-2/3 rounded-lg" />
      <div className="skeleton h-10 rounded-lg" />
    </DashboardShell>
  );
}

/** Album-style cover: a mosaic of representative photos with the headline
 * count, date range and a couple of stats laid over a gradient. */
function CoverHero({
  assets,
  loading,
  source,
  count,
  range,
  liked,
  rated,
  locale,
}: {
  assets: Asset[];
  loading: boolean;
  source: WidgetProps["source"];
  count: number;
  range: string;
  liked: number;
  rated: number;
  locale?: string;
}) {
  const { t } = useI18n();
  const tiles = assets.slice(0, 3);

  return (
    <div className="relative h-40 overflow-hidden rounded-2xl bg-base-200">
      {loading ? (
        <div className="skeleton h-full w-full" />
      ) : tiles.length === 0 ? (
        <div className="h-full w-full bg-gradient-to-br from-primary/25 to-secondary/20" />
      ) : (
        <div className="grid h-full grid-cols-3 gap-0.5">
          <div className={tiles.length === 1 ? "col-span-3" : "col-span-2"}>
            <WidgetAssetThumbnail
              asset={tiles[0]}
              source={source}
              className="h-full w-full object-cover"
            />
          </div>
          {tiles.length > 1 && (
            <div className="grid grid-rows-2 gap-0.5">
              {tiles.slice(1, 3).map((asset, i) => (
                <WidgetAssetThumbnail
                  key={asset.asset_id ?? i}
                  asset={asset}
                  source={source}
                  className="h-full w-full object-cover"
                />
              ))}
            </div>
          )}
        </div>
      )}

      <div className="pointer-events-none absolute inset-0 bg-gradient-to-t from-black/70 via-black/10 to-transparent" />
      <div className="absolute inset-x-0 bottom-0 flex items-end justify-between gap-3 p-3 text-white">
        <div className="min-w-0">
          <div className="text-2xl font-semibold leading-tight">{compact(count, locale)}</div>
          <div className="truncate text-xs text-white/80">
            {t("lumilio.widgets.metrics.assets", "Assets")} · {range}
          </div>
        </div>
        <div className="flex shrink-0 gap-1.5">
          <CoverStat icon={<Heart size={12} />} value={liked} locale={locale} />
          <CoverStat icon={<Star size={12} />} value={rated} locale={locale} />
        </div>
      </div>
    </div>
  );
}

function CoverStat({ icon, value, locale }: { icon: ReactNode; value: number; locale?: string }) {
  if (value <= 0) return null;
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-white/20 px-2 py-1 text-xs font-medium backdrop-blur-sm">
      {icon}
      {compact(value, locale)}
    </span>
  );
}

function FacetSection({
  icon,
  label,
  hidden = false,
  children,
}: {
  icon?: ReactNode;
  label: string;
  hidden?: boolean;
  children: ReactNode;
}) {
  if (hidden) return null;
  return (
    <div className="space-y-2">
      <div className="flex items-center gap-1.5 text-xs font-medium text-base-content/55">
        {icon}
        <span>{label}</span>
      </div>
      {children}
    </div>
  );
}

function RankedChips({ values }: { values?: NameCount[] }) {
  const top = (values ?? []).slice(0, 4);
  if (top.length === 0) return <span className="text-xs text-base-content/40">—</span>;
  return (
    <div className="flex flex-wrap gap-1.5">
      {top.map((value, index) => (
        <span
          key={value.name}
          className="inline-flex items-center gap-1.5 rounded-full border border-base-300 bg-base-100 py-1 pl-1 pr-2.5 text-xs"
        >
          <span className="grid size-5 place-items-center rounded-full bg-primary/15 text-[10px] font-semibold text-primary">
            {index + 1}
          </span>
          <span className="font-medium text-base-content">{value.name}</span>
          <span className="text-base-content/45">{value.count}</span>
        </span>
      ))}
    </div>
  );
}

function AvatarStack({ values }: { values?: NameCount[] }) {
  const people = (values ?? []).slice(0, 6);
  if (people.length === 0) return null;
  const lead = people.slice(0, 5);
  const extra = people.length - lead.length;

  return (
    <div className="flex items-center gap-3">
      <div className="flex">
        {lead.map((person, i) => {
          const name = person.name ?? "";
          return (
            <span
              key={name || i}
              title={`${name} · ${person.count}`}
              className="grid size-9 -ml-2 place-items-center rounded-full text-xs font-semibold text-white ring-2 ring-base-100 first:ml-0"
              style={{ backgroundColor: avatarColor(name) }}
            >
              {initials(name)}
            </span>
          );
        })}
        {extra > 0 && (
          <span className="grid size-9 -ml-2 place-items-center rounded-full bg-base-300 text-xs font-medium text-base-content/60 ring-2 ring-base-100">
            +{extra}
          </span>
        )}
      </div>
      <span className="min-w-0 truncate text-xs text-base-content/55">
        {people
          .slice(0, 2)
          .map((p) => p.name)
          .join(", ")}
      </span>
    </div>
  );
}

function initials(name: string): string {
  const trimmed = name.trim();
  if (!trimmed) return "?";
  // CJK: a single character reads better than two.
  if (/[㐀-鿿豈-﫿぀-ヿ]/.test(trimmed)) return trimmed.slice(-1);
  const parts = trimmed.split(/\s+/);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

function avatarColor(name: string): string {
  let hash = 0;
  for (let i = 0; i < name.length; i += 1) hash = name.charCodeAt(i) + ((hash << 5) - hash);
  return `hsl(${Math.abs(hash) % 360} 52% 45%)`;
}

function formatDateRange(facets: Facets, locale?: string): string {
  if (!facets.date_range?.from || !facets.date_range?.to) return "—";
  const granularity = facets.histogram_granularity ?? inferFacetGranularity(facets.histogram ?? []);
  const options: Intl.DateTimeFormatOptions =
    granularity === "year"
      ? { year: "numeric" }
      : granularity === "month"
        ? { year: "numeric", month: "short" }
        : { year: "numeric", month: "short", day: "numeric" };
  const format = new Intl.DateTimeFormat(locale, options);
  const from = format.format(new Date(facets.date_range.from));
  const to = format.format(new Date(facets.date_range.to));
  return from === to ? from : `${from} – ${to}`;
}
