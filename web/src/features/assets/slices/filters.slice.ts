import { StateCreator } from "zustand";
import { FiltersState } from "../types/assets.type";
import type { AssetFilter } from "../types/assets.type";
import { mapFilenameModeToDTO } from "../utils/filterUtils";

export interface FiltersSlice {
  filters: FiltersState;
  setFiltersEnabled: (enabled: boolean) => void;
  setFilterType: (type: FiltersState["type"]) => void;
  setFilterRaw: (raw: boolean | undefined) => void;
  setFilterRating: (rating: number | undefined) => void;
  setFilterLiked: (liked: boolean | undefined) => void;
  setFilterFilename: (filename: FiltersState["filename"]) => void;
  setFilterDate: (date: FiltersState["date"]) => void;
  setFilterCameraModel: (cameraModel: string | undefined) => void;
  setFilterLens: (lens: string | undefined) => void;
  setFilterTagNames: (tagNames: string[] | undefined) => void;
  setFilterLocation: (location: FiltersState["location"]) => void;
  resetFilters: () => void;
  batchUpdateFilters: (updates: Partial<FiltersState>) => void;
}

export const createFiltersSlice: StateCreator<
  FiltersSlice,
  [["zustand/immer", never]],
  [],
  FiltersSlice
> = (set) => ({
  filters: {
    enabled: false,
    type: undefined,
    raw: undefined,
    rating: undefined,
    liked: undefined,
    filename: undefined,
    date: undefined,
    camera_model: undefined,
    lens: undefined,
    tag_names: undefined,
    location: undefined,
  },

  setFiltersEnabled: (enabled) =>
    set((state) => {
      state.filters.enabled = enabled;
    }),

  setFilterType: (type) =>
    set((state) => {
      state.filters.type = type;
    }),

  setFilterRaw: (raw) =>
    set((state) => {
      state.filters.raw = raw;
    }),

  setFilterRating: (rating) =>
    set((state) => {
      state.filters.rating = rating;
    }),

  setFilterLiked: (liked) =>
    set((state) => {
      state.filters.liked = liked;
    }),

  setFilterFilename: (filename) =>
    set((state) => {
      state.filters.filename = filename;
    }),

  setFilterDate: (date) =>
    set((state) => {
      state.filters.date = date;
    }),

  setFilterCameraModel: (cameraModel) =>
    set((state) => {
      state.filters.camera_model = cameraModel;
    }),

  setFilterLens: (lens) =>
    set((state) => {
      state.filters.lens = lens;
    }),

  setFilterTagNames: (tagNames) =>
    set((state) => {
      state.filters.tag_names = tagNames;
    }),

  setFilterLocation: (location) =>
    set((state) => {
      state.filters.location = location;
    }),

  resetFilters: () =>
    set((state) => {
      state.filters = {
        enabled: false,
        type: undefined,
        raw: undefined,
        rating: undefined,
        liked: undefined,
        filename: undefined,
        date: undefined,
        camera_model: undefined,
        lens: undefined,
        tag_names: undefined,
        location: undefined,
      };
    }),

  batchUpdateFilters: (updates) =>
    set((state) => {
      const nextFilters = {
        ...state.filters,
        ...updates,
      };
      if (JSON.stringify(state.filters) === JSON.stringify(nextFilters)) {
        return;
      }
      state.filters = nextFilters;
    }),
});

// Selectors - work with both FiltersSlice (store) and FiltersState (legacy context)
type FiltersInput = FiltersSlice | FiltersState;

// Helper to normalize input
const getFiltersState = (input: FiltersInput): FiltersState => {
  if ("filters" in input && input.filters && "enabled" in input.filters) {
    return input.filters;
  }
  return input as FiltersState;
};

const hasLocationFilter = (location?: FiltersState["location"]): boolean =>
  !!location &&
  !(location.north === 0 && location.south === 0 && location.east === 0 && location.west === 0);

export const selectFiltersEnabled = (input: FiltersInput): boolean => {
  const state = getFiltersState(input);
  return state.enabled;
};

export const selectActiveFilterCount = (input: FiltersInput): number => {
  const state = getFiltersState(input);
  if (!state.enabled) return 0;

  const activeCriteria = [
    state.type === "PHOTO" || state.type === "VIDEO",
    state.raw !== undefined,
    state.rating !== undefined,
    state.liked !== undefined,
    state.filename?.value?.trim(),
    state.date && (state.date.from || state.date.to),
    state.camera_model?.trim(),
    state.lens?.trim(),
    state.tag_names && state.tag_names.length > 0,
    hasLocationFilter(state.location),
  ];

  return activeCriteria.filter(Boolean).length;
};

export const selectHasActiveFilters = (input: FiltersInput): boolean => {
  return selectActiveFilterCount(input) > 0;
};

export const selectFilterAsAssetFilter = (input: FiltersInput): AssetFilter => {
  const state = getFiltersState(input);
  if (!state.enabled) return {};

  const filter: AssetFilter = {};

  if (state.type === "PHOTO" || state.type === "VIDEO") {
    filter.type = state.type;
  }
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
      operator: mapFilenameModeToDTO(state.filename.mode),
      value: state.filename.value.trim(),
    };
  }
  if (state.date && (state.date.from || state.date.to)) {
    filter.date = {
      from: state.date.from,
      to: state.date.to,
    };
  }
  if (state.camera_model && state.camera_model.trim()) {
    filter.camera_model = state.camera_model.trim();
  }
  if (state.lens && state.lens.trim()) {
    filter.lens = state.lens.trim();
  }
  if (state.tag_names && state.tag_names.length > 0) {
    filter.tag_names = [...state.tag_names];
  }
  if (hasLocationFilter(state.location)) {
    filter.location = { ...state.location };
  }

  return filter;
};

export const selectIsFilterEmpty = (input: FiltersInput): boolean => {
  const assetFilter = selectFilterAsAssetFilter(input);
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
export const mergeFilters = (base: FiltersState, override: Partial<FiltersState>): FiltersState => {
  return {
    ...base,
    ...override,
    // Deep merge for complex fields
    filename: override.filename !== undefined ? override.filename : base.filename,
    date: override.date !== undefined ? override.date : base.date,
  };
};
