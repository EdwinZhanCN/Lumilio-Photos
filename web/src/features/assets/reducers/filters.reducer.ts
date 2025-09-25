import { AssetsAction, FiltersState } from "../types";

export const initialFiltersState: FiltersState = {
  enabled: false,
  raw: undefined,
  rating: undefined,
  liked: undefined,
  filename: undefined,
  date: undefined,
  camera_make: undefined,
  lens: undefined,
};

export const filtersReducer = (
  state: FiltersState = initialFiltersState,
  action: AssetsAction,
): FiltersState => {
  switch (action.type) {
    case "SET_FILTERS_ENABLED":
      return {
        ...state,
        enabled: action.payload,
      };

    case "SET_FILTER_RAW":
      return {
        ...state,
        raw: action.payload,
      };

    case "SET_FILTER_RATING":
      return {
        ...state,
        rating: action.payload,
      };

    case "SET_FILTER_LIKED":
      return {
        ...state,
        liked: action.payload,
      };

    case "SET_FILTER_FILENAME":
      return {
        ...state,
        filename: action.payload,
      };

    case "SET_FILTER_DATE":
      return {
        ...state,
        date: action.payload,
      };

    case "SET_FILTER_CAMERA_MAKE":
      return {
        ...state,
        camera_make: action.payload,
      };

    case "SET_FILTER_LENS":
      return {
        ...state,
        lens: action.payload,
      };

    case "RESET_FILTERS":
      return {
        ...initialFiltersState,
      };

    case "BATCH_UPDATE_FILTERS":
      return {
        ...state,
        ...action.payload,
      };

    default:
      return state;
  }
};

// Selectors
export const selectFiltersEnabled = (state: FiltersState): boolean => state.enabled;

export const selectActiveFilterCount = (state: FiltersState): number => {
  if (!state.enabled) return 0;

  let count = 0;
  if (state.raw !== undefined) count++;
  if (state.rating !== undefined) count++;
  if (state.liked !== undefined) count++;
  if (state.filename && state.filename.value.trim()) count++;
  if (state.date && (state.date.from || state.date.to)) count++;
  if (state.camera_make && state.camera_make.trim()) count++;
  if (state.lens && state.lens.trim()) count++;

  return count;
};

export const selectHasActiveFilters = (state: FiltersState): boolean => {
  return selectActiveFilterCount(state) > 0;
};

export const selectFilterAsAssetFilter = (state: FiltersState) => {
  if (!state.enabled) return {};

  const filter: any = {};

  if (state.raw !== undefined) {
    filter.raw = state.raw;
  }
  if (state.rating !== undefined) {
    filter.rating = state.rating;
  }
  if (state.liked !== undefined) {
    filter.liked = state.liked;
  }
  if (state.filename && state.filename.value.trim()) {
    filter.filename = {
      mode: state.filename.mode,
      value: state.filename.value.trim(),
    };
  }
  if (state.date && (state.date.from || state.date.to)) {
    filter.date = {
      from: state.date.from,
      to: state.date.to,
    };
  }
  if (state.camera_make && state.camera_make.trim()) {
    filter.camera_make = state.camera_make.trim();
  }
  if (state.lens && state.lens.trim()) {
    filter.lens = state.lens.trim();
  }

  return filter;
};

export const selectIsFilterEmpty = (state: FiltersState): boolean => {
  const assetFilter = selectFilterAsAssetFilter(state);
  return Object.keys(assetFilter).length === 0;
};

// Filter validation utilities
export const validateFilters = (filters: Partial<FiltersState>): string[] => {
  const errors: string[] = [];

  if (filters.rating !== undefined) {
    if (filters.rating < 0 || filters.rating > 5) {
      errors.push("Rating must be between 0 and 5");
    }
  }

  if (filters.filename) {
    if (!filters.filename.value || filters.filename.value.trim().length === 0) {
      errors.push("Filename filter value cannot be empty");
    }
  }

  if (filters.date) {
    const { from, to } = filters.date;
    if (from && to && new Date(from) > new Date(to)) {
      errors.push("Date 'from' cannot be later than 'to'");
    }
  }

  return errors;
};

// Filter transformation utilities
export const mergeFilters = (
  base: FiltersState,
  override: Partial<FiltersState>,
): FiltersState => {
  return {
    ...base,
    ...override,
    // Deep merge for complex fields
    filename: override.filename !== undefined ? override.filename : base.filename,
    date: override.date !== undefined ? override.date : base.date,
  };
};
