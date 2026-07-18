export type FilenameOperator = "contains" | "matches" | "starts_with" | "ends_with";
export type MediaTypeFilter = "PHOTO" | "VIDEO";

export interface FilenameFilter {
  operator: FilenameOperator;
  value: string;
}

export interface DateRange {
  from?: string;
  to?: string;
}

export interface LocationBBox {
  north: number;
  south: number;
  east: number;
  west: number;
}

export interface FilterDTO {
  type?: MediaTypeFilter;
  raw?: boolean;
  rating?: number;
  liked?: boolean;
  filename?: FilenameFilter;
  date?: DateRange;
  camera_model?: string;
  lens?: string;
  tag_names?: string[];
  location?: LocationBBox;
}

export type FilterFieldKey = keyof FilterDTO;

export interface FilterToolProps {
  initial?: FilterDTO;
  onChange?: (filters: FilterDTO) => void;
  autoApply?: boolean;
  lockedFields?: readonly FilterFieldKey[] | Partial<Record<FilterFieldKey, boolean>>;
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
  location: LocationBBox;
  cameraModelEnabled: boolean;
  cameraModel: string;
  lensEnabled: boolean;
  lens: string;
  tagEnabled: boolean;
  tagNames: string[];
}
