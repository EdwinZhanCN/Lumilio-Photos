import { StateCreator } from "zustand";
import { FiltersState } from "../types/assets.type";
import { mapFilenameModeToDTO } from "../utils/filterUtils";

export interface FiltersSlice {
  filters: FiltersState;
  setFiltersEnabled: (enabled: boolean) => void;
  setFilterRaw: (raw: boolean | undefined) => void;
  setFilterRating: (rating: number | undefined) => void;
  setFilterLiked: (liked: boolean | undefined) => void;
  setFilterFilename: (filename: FiltersState["filename"]) => void;
  setFilterDate: (date: FiltersState["date"]) => void;
  setFilterCameraMake: (cameraMake: string | undefined) => void;
  setFilterLens: (lens: string | undefined) => void;
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
    raw: undefined,
    rating: undefined,
    liked: undefined,
    filename: undefined,
    date: undefined,
    camera_make: undefined,
    lens: undefined,
  },

  setFiltersEnabled: (enabled) =>
    set((state) => {
      state.filters.enabled = enabled;
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

  setFilterCameraMake: (cameraMake) =>
    set((state) => {
      state.filters.camera_make = cameraMake;
    }),

  setFilterLens: (lens) =>
    set((state) => {
      state.filters.lens = lens;
    }),

  resetFilters: () =>
    set((state) => {
      state.filters = {
        enabled: false,
        raw: undefined,
        rating: undefined,
        liked: undefined,
        filename: undefined,
        date: undefined,
        camera_make: undefined,
        lens: undefined,
      };
    }),

  batchUpdateFilters: (updates) =>
    set((state) => {
      Object.assign(state.filters, updates);
    }),
});

// Selectors - work with both FiltersSlice (store) and FiltersState (legacy context)
type FiltersInput = FiltersSlice | FiltersState;

// Helper to normalize input
const getFiltersState = (input: FiltersInput): FiltersState => {
  if ('filters' in input && input.filters && 'enabled' in input.filters) {
    return input.filters;
  }
  return input as FiltersState;
};

export const selectFiltersEnabled = (input: FiltersInput): boolean => {
  const state = getFiltersState(input);
  return state.enabled;
};

export const selectActiveFilterCount = (input: FiltersInput): number => {
  const state = getFiltersState(input);
  if (!state.enabled) return 0;

  const activeCriteria = [
    state.raw !== undefined,
    state.rating !== undefined,
    state.liked !== undefined,
    state.filename?.value?.trim(),
    state.date && (state.date.from || state.date.to),
    state.camera_make?.trim(),
    state.lens?.trim(),
  ];

  return activeCriteria.filter(Boolean).length;
};

export const selectHasActiveFilters = (input: FiltersInput): boolean => {
  return selectActiveFilterCount(input) > 0;
};

export const selectFilterAsAssetFilter = (input: FiltersInput) => {
  const state = getFiltersState(input);
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
  if (state.camera_make && state.camera_make.trim()) {
    filter.camera_make = state.camera_make.trim();
  }
  if (state.lens && state.lens.trim()) {
    filter.lens = state.lens.trim();
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
export const mergeFilters = (
  base: FiltersState,
  override: Partial<FiltersState>,
): FiltersState => {
  return {
    ...base,
    ...override,
    // Deep merge for complex fields
    filename:
      override.filename !== undefined ? override.filename : base.filename,
    date: override.date !== undefined ? override.date : base.date,
  };
};
