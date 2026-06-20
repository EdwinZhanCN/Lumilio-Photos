import { createPortal } from "react-dom";
import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Sparkles } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { useI18n } from "@/lib/i18n.tsx";
import type { WidgetProps } from "./types";
import { useWidgetAssetsPreview } from "./useWidgetAssets";

const INLINE_STORY_COUNT = 6;
const BOARD_STORY_COUNT = 14;

export function StorylineWidget({
  source,
  variant,
  count,
  title,
}: WidgetProps) {
  const { t } = useI18n();
  const limit = variant === "board" ? BOARD_STORY_COUNT : INLINE_STORY_COUNT;
  const { assets, isLoading, isError } = useWidgetAssetsPreview(source, limit);
  const [carouselAssetId, setCarouselAssetId] = useState<string>();

  const slideIndex = useMemo(
    () =>
      carouselAssetId
        ? assets.findIndex((asset) => asset.asset_id === carouselAssetId)
        : -1,
    [assets, carouselAssetId],
  );

  if (isLoading) return <StoryShell variant={variant} title={title} loading />;
  if (isError || assets.length === 0) {
    return (
      <StoryShell variant={variant} title={title}>
        <div className="flex h-24 items-center justify-center text-center text-xs text-base-content/50">
          {t(
            "lumilio.widgets.storylineUnavailable",
            "This storyline is unavailable.",
          )}
        </div>
      </StoryShell>
    );
  }

  return (
    <StoryShell variant={variant} title={title}>
      <div className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-2 text-xs font-medium text-base-content/60">
          <Sparkles size={14} />
          <span className="truncate">
            {t("lumilio.widgets.storyline.sequence", "Sequence preview")}
          </span>
        </div>
        <span className="badge badge-outline badge-sm shrink-0">
          {t("lumilio.widgets.storyline.assets", "{{count}} assets", { count })}
        </span>
      </div>

      <div
        className={
          variant === "board"
            ? "grid grid-cols-3 gap-1.5"
            : "grid max-w-2xl grid-cols-3 gap-1.5"
        }
      >
        {assets.map((asset, index) => (
          <StoryTile
            key={asset.asset_id}
            asset={asset}
            index={index}
            featured={variant === "board" && index === 0}
            onOpen={() => setCarouselAssetId(asset.asset_id)}
          />
        ))}
      </div>

      {carouselAssetId &&
        assets.length > 0 &&
        createPortal(
          <FullScreenCarousel
            photos={assets}
            initialSlide={slideIndex >= 0 ? slideIndex : 0}
            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
            onClose={() => setCarouselAssetId(undefined)}
            onNavigate={setCarouselAssetId}
          />,
          document.body,
        )}
    </StoryShell>
  );
}

function StoryShell({
  variant,
  title,
  loading = false,
  children,
}: {
  variant: WidgetProps["variant"];
  title?: string;
  loading?: boolean;
  children?: ReactNode;
}) {
  return (
    <div
      className={
        variant === "board"
          ? "h-full overflow-y-auto p-3 space-y-3"
          : "my-3 max-w-2xl rounded-xl border border-base-300 bg-base-100 p-3 space-y-3"
      }
    >
      {variant === "inline" && title && (
        <div className="text-sm font-medium text-base-content/80">{title}</div>
      )}
      {loading ? (
        <div className="grid grid-cols-3 gap-1.5">
          {Array.from({ length: variant === "board" ? 9 : 6 }).map(
            (_, index) => (
              <div key={index} className="skeleton aspect-square rounded-lg" />
            ),
          )}
        </div>
      ) : (
        children
      )}
    </div>
  );
}

function StoryTile({
  asset,
  index,
  featured,
  onOpen,
}: {
  asset: Asset;
  index: number;
  featured: boolean;
  onOpen: () => void;
}) {
  return (
    <button
      className={
        featured
          ? "group relative col-span-2 row-span-2 aspect-square overflow-hidden rounded-lg bg-base-200 text-left"
          : "group relative aspect-square overflow-hidden rounded-lg bg-base-200 text-left"
      }
      onClick={onOpen}
    >
      <img
        src={assetUrls.getThumbnailUrl(asset.asset_id!, "small")}
        alt=""
        loading="lazy"
        className="h-full w-full object-cover transition-transform duration-200 group-hover:scale-105"
      />
      <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/65 to-transparent px-2 pb-2 pt-6 text-white">
        <div className="flex items-end justify-between gap-2">
          <span className="rounded-full bg-white/20 px-1.5 py-0.5 text-[10px] font-medium">
            {index + 1}
          </span>
          <span className="truncate text-[10px] text-white/80">
            {formatAssetDate(asset)}
          </span>
        </div>
      </div>
    </button>
  );
}

function formatAssetDate(asset: Asset): string {
  const rawDate = asset.taken_time ?? asset.upload_time;
  if (!rawDate) return "";
  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
  }).format(new Date(rawDate));
}
