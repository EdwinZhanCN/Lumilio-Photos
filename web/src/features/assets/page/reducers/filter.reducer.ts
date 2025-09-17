export type FilenameOperator =
  | "contains"
  | "matches"
  | "starts_with"
  | "ends_with";

export interface RawSlice {
  enabled: boolean;
  mode: "include" | "exclude"; // include => raw = true, exclude => raw = false
}

export interface RatingSlice {
  enabled: boolean;
  value: number; // 0-5 (0 = unrated sentinel)
}

export interface LikedSlice {
  enabled: boolean;
  value: boolean; // true = liked, false = unliked
}

export interface FilenameSlice {
  enabled: boolean;
  operator: FilenameOperator;
  value: string;
}

export interface DateSlice {
  enabled: boolean;
  from?: string; // YYYY-MM-DD
  to?: string; // YYYY-MM-DD
}

export interface LocationBBox {
  north: number;
  south: number;
  east: number;
  west: number;
}

export interface LocationSlice {
  enabled: boolean;
  bbox: LocationBBox;
}

export interface CameraMakeSlice {
  enabled: boolean;
  value: string;
}

export interface LensSlice {
  enabled: boolean;
  value: string;
}

export interface FilterState {
  enabled: boolean; // 全局启用总开关
  raw: RawSlice;
  rating: RatingSlice;
  liked: LikedSlice;
  filename: FilenameSlice;
  date: DateSlice;
  location: LocationSlice;
  cameraMake: CameraMakeSlice;
  lens: LensSlice;
  version: number; // 每次 reducer 更新时自增（便于缓存 / debug）
}

export interface BackendFilenameFilter {
  mode: "contains" | "matches" | "startswith" | "endswith";
  value: string;
}

export interface BackendDateRange {
  from?: string;
  to?: string;
}

export interface FilterDTO {
  raw?: boolean;
  rating?: number;
  liked?: boolean;
  filename?: BackendFilenameFilter;
  date?: BackendDateRange;
  camera_make?: string;
  lens?: string;
  // location（预留）：等后端协议确定后可以加入
}

export const FILTER_INITIAL_STATE: FilterState = {
  enabled: false,
  raw: { enabled: false, mode: "include" },
  rating: { enabled: false, value: 5 },
  liked: { enabled: false, value: true },
  filename: { enabled: false, operator: "contains", value: "" },
  date: { enabled: false, from: undefined, to: undefined },
  location: {
    enabled: false,
    bbox: { north: 0, south: 0, east: 0, west: 0 },
  },
  cameraMake: { enabled: false, value: "" },
  lens: { enabled: false, value: "" },
  version: 0,
};

export type FilterAction =
  | { type: "FILTER_TOGGLE_GLOBAL"; payload: boolean }
  | { type: "FILTER_RESET_ALL" }
  | { type: "FILTER_RAW_SET_ENABLED"; payload: boolean }
  | { type: "FILTER_RAW_SET_MODE"; payload: "include" | "exclude" }
  | { type: "FILTER_RATING_SET_ENABLED"; payload: boolean }
  | { type: "FILTER_RATING_SET_VALUE"; payload: number }
  | { type: "FILTER_LIKED_SET_ENABLED"; payload: boolean }
  | { type: "FILTER_LIKED_SET_VALUE"; payload: boolean }
  | { type: "FILTER_FILENAME_SET_ENABLED"; payload: boolean }
  | {
      type: "FILTER_FILENAME_UPDATE";
      payload: { operator?: FilenameOperator; value?: string };
    }
  | { type: "FILTER_DATE_SET_ENABLED"; payload: boolean }
  | { type: "FILTER_DATE_UPDATE"; payload: { from?: string; to?: string } }
  | { type: "FILTER_LOCATION_SET_ENABLED"; payload: boolean }
  | { type: "FILTER_LOCATION_UPDATE_BBOX"; payload: Partial<LocationBBox> }
  | { type: "FILTER_CAMERA_SET_ENABLED"; payload: boolean }
  | { type: "FILTER_CAMERA_SET_VALUE"; payload: string }
  | { type: "FILTER_LENS_SET_ENABLED"; payload: boolean }
  | { type: "FILTER_LENS_SET_VALUE"; payload: string }
  | { type: "FILTER_BATCH_PATCH"; payload: Partial<FilterState> }
  | { type: "FILTER_INTERNAL_VERSION_BUMP" };

export function isZeroBBox(b: LocationBBox): boolean {
  return b.north === 0 && b.south === 0 && b.east === 0 && b.west === 0;
}

export function mapFilenameOperatorToBackend(
  op: FilenameOperator,
): BackendFilenameFilter["mode"] {
  switch (op) {
    case "starts_with":
      return "startswith";
    case "ends_with":
      return "endswith";
    default:
      return op;
  }
}

export function selectActiveFilterCount(state: FilterState): number {
  if (!state.enabled) return 0;
  let count = 0;
  if (state.raw.enabled) count++;
  if (state.rating.enabled) count++;
  if (state.liked.enabled) count++;
  if (state.filename.enabled && state.filename.value.trim()) count++;
  if (state.date.enabled && (state.date.from || state.date.to)) count++;
  if (state.location.enabled && !isZeroBBox(state.location.bbox)) count++;
  if (state.cameraMake.enabled && state.cameraMake.value.trim()) count++;
  if (state.lens.enabled && state.lens.value.trim()) count++;
  return count;
}

export function buildFilterDTO(state: FilterState): FilterDTO {
  if (!state.enabled) return {};
  const dto: FilterDTO = {};

  if (state.raw.enabled) {
    dto.raw = state.raw.mode === "include";
  }
  if (state.rating.enabled) {
    dto.rating = state.rating.value;
  }
  if (state.liked.enabled) {
    dto.liked = state.liked.value;
  }
  if (state.filename.enabled && state.filename.value.trim()) {
    dto.filename = {
      mode: mapFilenameOperatorToBackend(state.filename.operator),
      value: state.filename.value.trim(),
    };
  }
  if (state.date.enabled && (state.date.from || state.date.to)) {
    dto.date = {
      from: state.date.from || undefined,
      to: state.date.to || undefined,
    };
  }
  if (state.cameraMake.enabled && state.cameraMake.value.trim()) {
    dto.camera_make = state.cameraMake.value.trim();
  }
  if (state.lens.enabled && state.lens.value.trim()) {
    dto.lens = state.lens.value.trim();
  }

  // location 暂未加入
  return dto;
}

export function selectIsFilterEffectiveEmpty(state: FilterState): boolean {
  return Object.keys(buildFilterDTO(state)).length === 0;
}

export function filterReducer(
  state: FilterState = FILTER_INITIAL_STATE,
  action: FilterAction,
): FilterState {
  switch (action.type) {
    case "FILTER_TOGGLE_GLOBAL":
      return {
        ...state,
        enabled: action.payload,
        version: state.version + 1,
      };

    case "FILTER_RESET_ALL":
      return {
        ...FILTER_INITIAL_STATE,
        version: state.version + 1,
      };

    case "FILTER_RAW_SET_ENABLED":
      return {
        ...state,
        raw: { ...state.raw, enabled: action.payload },
        version: state.version + 1,
      };
    case "FILTER_RAW_SET_MODE":
      return {
        ...state,
        raw: { ...state.raw, mode: action.payload },
        version: state.version + 1,
      };

    case "FILTER_RATING_SET_ENABLED":
      return {
        ...state,
        rating: { ...state.rating, enabled: action.payload },
        version: state.version + 1,
      };
    case "FILTER_RATING_SET_VALUE":
      return {
        ...state,
        rating: { ...state.rating, value: action.payload },
        version: state.version + 1,
      };

    case "FILTER_LIKED_SET_ENABLED":
      return {
        ...state,
        liked: { ...state.liked, enabled: action.payload },
        version: state.version + 1,
      };
    case "FILTER_LIKED_SET_VALUE":
      return {
        ...state,
        liked: { ...state.liked, value: action.payload },
        version: state.version + 1,
      };

    case "FILTER_FILENAME_SET_ENABLED":
      return {
        ...state,
        filename: { ...state.filename, enabled: action.payload },
        version: state.version + 1,
      };
    case "FILTER_FILENAME_UPDATE":
      return {
        ...state,
        filename: {
          ...state.filename,
          operator: action.payload.operator ?? state.filename.operator,
          value: action.payload.value ?? state.filename.value,
        },
        version: state.version + 1,
      };

    case "FILTER_DATE_SET_ENABLED":
      return {
        ...state,
        date: { ...state.date, enabled: action.payload },
        version: state.version + 1,
      };
    case "FILTER_DATE_UPDATE":
      return {
        ...state,
        date: {
          ...state.date,
          from: action.payload.from ?? state.date.from,
          to: action.payload.to ?? state.date.to,
        },
        version: state.version + 1,
      };

    case "FILTER_LOCATION_SET_ENABLED":
      return {
        ...state,
        location: { ...state.location, enabled: action.payload },
        version: state.version + 1,
      };
    case "FILTER_LOCATION_UPDATE_BBOX":
      return {
        ...state,
        location: {
          ...state.location,
          bbox: { ...state.location.bbox, ...action.payload },
        },
        version: state.version + 1,
      };

    case "FILTER_CAMERA_SET_ENABLED":
      return {
        ...state,
        cameraMake: { ...state.cameraMake, enabled: action.payload },
        version: state.version + 1,
      };
    case "FILTER_CAMERA_SET_VALUE":
      return {
        ...state,
        cameraMake: { ...state.cameraMake, value: action.payload },
        version: state.version + 1,
      };

    case "FILTER_LENS_SET_ENABLED":
      return {
        ...state,
        lens: { ...state.lens, enabled: action.payload },
        version: state.version + 1,
      };
    case "FILTER_LENS_SET_VALUE":
      return {
        ...state,
        lens: { ...state.lens, value: action.payload },
        version: state.version + 1,
      };

    case "FILTER_BATCH_PATCH":
      return {
        ...state,
        ...action.payload,
        version: state.version + 1,
      };

    case "FILTER_INTERNAL_VERSION_BUMP":
      return {
        ...state,
        version: state.version + 1,
      };

    default:
      return state;
  }
}

export function serializeFilterState(state: FilterState): FilterState {
  return JSON.parse(JSON.stringify(state));
}

export function hydrateFilterState(partial: Partial<FilterState>): FilterState {
  return {
    ...FILTER_INITIAL_STATE,
    ...partial,
    raw: { ...FILTER_INITIAL_STATE.raw, ...(partial.raw ?? {}) },
    rating: { ...FILTER_INITIAL_STATE.rating, ...(partial.rating ?? {}) },
    liked: { ...FILTER_INITIAL_STATE.liked, ...(partial.liked ?? {}) },
    filename: {
      ...FILTER_INITIAL_STATE.filename,
      ...(partial.filename ?? {}),
    },
    date: { ...FILTER_INITIAL_STATE.date, ...(partial.date ?? {}) },
    location: {
      ...FILTER_INITIAL_STATE.location,
      ...(partial.location ?? {}),
      bbox: {
        ...FILTER_INITIAL_STATE.location.bbox,
        ...(partial.location?.bbox ?? {}),
      },
    },
    cameraMake: {
      ...FILTER_INITIAL_STATE.cameraMake,
      ...(partial.cameraMake ?? {}),
    },
    lens: { ...FILTER_INITIAL_STATE.lens, ...(partial.lens ?? {}) },
    version: (partial.version ?? 0) + 1,
  };
}

export function centerToBBox(
  lat: number,
  lon: number,
  radiusKm: number,
): LocationBBox {
  const dLat = radiusKm / 110.574;
  const dLon = radiusKm / (111.32 * Math.cos((lat * Math.PI) / 180));
  return {
    north: lat + dLat,
    south: lat - dLat,
    east: lon + dLon,
    west: lon - dLon,
  };
}

export const filterSelectors = {
  buildDTO: buildFilterDTO,
  activeCount: selectActiveFilterCount,
  isEmpty: selectIsFilterEffectiveEmpty,
};
