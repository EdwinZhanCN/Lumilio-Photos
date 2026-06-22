import { ImageIcon } from "lucide-react";
import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";
import { isMockWidgetSource } from "./mockWidgetData";
import type { WidgetSource } from "./types";

export function WidgetAssetThumbnail({
  asset,
  source,
  className,
}: {
  asset: Asset;
  source: WidgetSource;
  className: string;
}) {
  if (isMockWidgetSource(source)) {
    return (
      <div className={`${className} grid place-items-center bg-base-200 text-base-content/35`}>
        <ImageIcon size={22} strokeWidth={1.5} />
      </div>
    );
  }

  return (
    <img
      src={assetUrls.getThumbnailUrl(asset.asset_id!, "small")}
      alt=""
      loading="lazy"
      className={className}
    />
  );
}
