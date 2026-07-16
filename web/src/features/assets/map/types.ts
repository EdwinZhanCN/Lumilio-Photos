import type { LatLngExpression, LatLngTuple } from "leaflet";
import type { BBox } from "supercluster";

import type { Asset } from "@/lib/assets/types";

export type PhotoLocation = {
  id: string;
  position: LatLngExpression;
  title: string;
  description?: string;
  thumbnailUrl?: string;
  asset?: Asset;
};

export type MapBoundsOverlay = {
  north: number;
  south: number;
  east: number;
  west: number;
};

export type MapViewport = {
  bbox: BBox;
  zoom: number;
};

export interface MapComponentProps {
  photoLocations?: PhotoLocation[];
  onPointClick?: (assetId: string) => void;
  center?: LatLngTuple;
  zoom?: number;
  height?: string | number;
  className?: string;
  rounded?: boolean;
  showSinglePhoto?: boolean;
  boundsOverlay?: MapBoundsOverlay;
  onViewportChange?: (viewport: MapViewport) => void;
}
