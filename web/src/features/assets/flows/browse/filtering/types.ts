import type {
  AssetLocationBBox,
  AssetUserFilter,
  AssetUserFilterKey,
  MediaComposition,
  StackKind,
  StackMembership,
} from "../../../model/filter";

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

/**
 * Flat draft: a value's presence is what makes a section active. `undefined` (or an
 * empty string / empty array) means "all", so there is no separate enabled flag.
 */
export interface FilterDraft {
  type?: MediaTypeFilter;
  composition?: MediaComposition;
  stackMembership?: StackMembership;
  stackKinds: StackKind[];
  rating?: number;
  liked?: boolean;
  filenameOperator: FilenameOperator;
  filenameValue: string;
  dateFrom: string;
  dateTo: string;
  location?: AssetLocationBBox;
  cameraModel: string;
  lens: string;
  tagNames: string[];
}
