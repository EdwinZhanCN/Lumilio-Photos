import type { AssetLocationBBox, AssetUserFilter, AssetUserFilterKey } from "../../../model/filter";

export type FilenameOperator = NonNullable<AssetUserFilter["filename"]>["operator"];
export type MediaTypeFilter = NonNullable<AssetUserFilter["type"]>;

export interface FilterToolProps {
  initial?: AssetUserFilter;
  onChange?: (filters: AssetUserFilter) => void;
  autoApply?: boolean;
  lockedFields?: readonly AssetUserFilterKey[];
  cameraModelOptions?: string[];
  lensOptions?: string[];
  fetchCameraModels?: () => Promise<string[]>;
  fetchLenses?: () => Promise<string[]>;
}

export interface FilterDraft {
  filterEnabled: boolean;
  typeEnabled: boolean;
  typeValue: MediaTypeFilter;
  rawEnabled: boolean;
  rawMode: "include" | "exclude";
  ratingEnabled: boolean;
  ratingValue: number;
  likedEnabled: boolean;
  likedValue: boolean;
  filenameEnabled: boolean;
  filenameOperator: FilenameOperator;
  filenameValue: string;
  dateEnabled: boolean;
  dateFrom: string;
  dateTo: string;
  locationEnabled: boolean;
  location: AssetLocationBBox;
  cameraModelEnabled: boolean;
  cameraModel: string;
  lensEnabled: boolean;
  lens: string;
  tagEnabled: boolean;
  tagNames: string[];
}
