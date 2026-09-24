import { assetUrls } from "@/lib/assets/assetUrls";
import type { Asset } from "@/lib/assets/types";

export function WidgetAssetThumbnail({ asset, className }: { asset: Asset; className: string }) {
  return (
    <img
      src={assetUrls.getThumbnailUrl(asset.asset_id!, "small")}
      alt=""
      loading="lazy"
      className={className}
    />
  );
}
