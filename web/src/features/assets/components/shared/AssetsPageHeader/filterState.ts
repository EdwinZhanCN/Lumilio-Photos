import { mapFilenameModeToDTO, mapFilenameOperatorToMode } from "../../../utils/filterUtils";
import type { FiltersState } from "../../../types/assets.type";
import type { FilterDTO } from "../../page/FilterTool/types";

export function filtersToDTO(filters: FiltersState): FilterDTO {
  if (!filters?.enabled) return {};

  const dto: FilterDTO = {};
  if (filters.type === "PHOTO" || filters.type === "VIDEO") dto.type = filters.type;
  if (typeof filters.raw === "boolean") dto.raw = filters.raw;
  if (typeof filters.rating === "number") dto.rating = filters.rating;
  if (typeof filters.liked === "boolean") dto.liked = filters.liked;
  if (filters.filename && filters.filename.value?.trim()) {
    const operator = mapFilenameModeToDTO(filters.filename.mode);
    dto.filename = {
      operator: operator!,
      value: filters.filename.value,
    };
  }
  if (filters.date && (filters.date.from || filters.date.to)) {
    dto.date = { from: filters.date.from, to: filters.date.to };
  }
  if (filters.camera_model?.trim()) dto.camera_model = filters.camera_model.trim();
  if (filters.lens?.trim()) dto.lens = filters.lens.trim();
  if (filters.tag_names && filters.tag_names.length > 0) dto.tag_names = [...filters.tag_names];
  if (filters.location) dto.location = { ...filters.location };
  return dto;
}

export function filterDTOToPayload(newFilters: FilterDTO): Partial<FiltersState> {
  const payload: Partial<FiltersState> = {
    enabled: Object.keys(newFilters).length > 0,
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

  if (newFilters.type === "PHOTO" || newFilters.type === "VIDEO") {
    payload.type = newFilters.type;
  }
  if (newFilters.raw !== undefined) payload.raw = newFilters.raw;
  if (newFilters.rating !== undefined) payload.rating = newFilters.rating;
  if (newFilters.liked !== undefined) payload.liked = newFilters.liked;

  if (newFilters.filename && newFilters.filename.value.trim()) {
    payload.filename = {
      mode: mapFilenameOperatorToMode(newFilters.filename.operator)!,
      value: newFilters.filename.value.trim(),
    };
  }

  if (newFilters.date && (newFilters.date.from || newFilters.date.to)) {
    payload.date = {
      from: newFilters.date.from,
      to: newFilters.date.to,
    };
  }

  if (newFilters.camera_model && newFilters.camera_model.trim()) {
    payload.camera_model = newFilters.camera_model.trim();
  }

  if (newFilters.lens && newFilters.lens.trim()) {
    payload.lens = newFilters.lens.trim();
  }

  if (newFilters.tag_names && newFilters.tag_names.length > 0) {
    payload.tag_names = newFilters.tag_names;
  }

  if (newFilters.location) {
    payload.location = { ...newFilters.location };
  }

  return payload;
}
