import { ListFilterIcon } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n";
import {
  areLocationBBoxesEqual,
  buildFilterDTO,
  buildLockedInitialDTO,
  countEnabledFilters,
  createLockedFieldSet,
  EMPTY_LOCATION_BBOX,
  hasActiveLockedFields,
  isFieldActive,
  toDateInput,
} from "./filterState";
import { LikeSection, RatingSection, RawSection, TypeSection } from "./sections/ChoiceSections";
import { LocationSection } from "./sections/LocationSection";
import { CameraMakeSection, LensSection, TagSection } from "./sections/MetadataSections";
import { DateSection, FilenameSection } from "./sections/ValueSections";
import type {
  FilterDraft,
  FilterFieldKey,
  FilterToolProps,
  FilenameOperator,
  LocationBBox,
  MediaTypeFilter,
} from "./types";
import { useFilterOptions } from "./useFilterOptions";

export type {
  DateRange,
  FilenameFilter,
  FilenameOperator,
  FilterDTO,
  FilterFieldKey,
  LocationBBox,
  MediaTypeFilter,
} from "./types";

export default function FilterTool({
  initial,
  onChange,
  autoApply = true,
  lockedFields,
  cameraModelOptions,
  lensOptions,
  fetchCameraModels,
  fetchLenses,
}: FilterToolProps) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const initialDTO = useMemo(() => initial ?? {}, [initial]);
  const initialHash = useMemo(() => JSON.stringify(initialDTO), [initialDTO]);
  const lastSyncedInitialHashRef = useRef(initialHash);
  const lastAutoAppliedHashRef = useRef("");

  const lockedFieldSet = useMemo(() => createLockedFieldSet(lockedFields), [lockedFields]);
  const lockedFieldsHash = useMemo(
    () => Array.from(lockedFieldSet).sort().join("|"),
    [lockedFieldSet],
  );
  const isFieldLocked = useCallback(
    (field: FilterFieldKey) => lockedFieldSet.has(field),
    [lockedFieldSet],
  );
  const hasLockedInitialFilters = useMemo(
    () => hasActiveLockedFields(initialDTO, lockedFieldSet),
    [initialDTO, initialHash, lockedFieldSet, lockedFieldsHash],
  );

  const [filterEnabled, setFilterEnabled] = useState(
    Object.keys(initialDTO).length > 0 || hasLockedInitialFilters,
  );

  const [typeEnabled, setTypeEnabled] = useState(
    initialDTO.type === "PHOTO" || initialDTO.type === "VIDEO",
  );
  const [typeValue, setTypeValue] = useState<MediaTypeFilter>(
    initialDTO.type === "VIDEO" ? "VIDEO" : "PHOTO",
  );

  const [rawEnabled, setRawEnabled] = useState(typeof initialDTO.raw === "boolean");
  const [rawMode, setRawMode] = useState<"include" | "exclude">(
    initialDTO.raw === false ? "exclude" : "include",
  );

  const [ratingEnabled, setRatingEnabled] = useState(typeof initialDTO.rating === "number");
  const [ratingValue, setRatingValue] = useState(
    typeof initialDTO.rating === "number" ? initialDTO.rating : 5,
  );

  const [likedEnabled, setLikedEnabled] = useState(typeof initialDTO.liked === "boolean");
  const [likedValue, setLikedValue] = useState(initialDTO.liked ?? true);

  const [filenameEnabled, setFilenameEnabled] = useState(!!initialDTO.filename);
  const [filenameOperator, setFilenameOperator] = useState<FilenameOperator>(
    initialDTO.filename?.operator ?? "contains",
  );
  const [filenameValue, setFilenameValue] = useState(initialDTO.filename?.value ?? "");

  const [dateEnabled, setDateEnabled] = useState(!!initialDTO.date);
  const [dateFrom, setDateFrom] = useState(toDateInput(initialDTO.date?.from ?? ""));
  const [dateTo, setDateTo] = useState(toDateInput(initialDTO.date?.to ?? ""));

  const [locationEnabled, setLocationEnabled] = useState(!!initialDTO.location);
  const [location, setLocation] = useState<LocationBBox>(
    initialDTO.location ?? EMPTY_LOCATION_BBOX,
  );

  const [cameraModelEnabled, setCameraModelEnabled] = useState(!!initialDTO.camera_model);
  const [cameraModel, setCameraModel] = useState(initialDTO.camera_model ?? "");
  const [lensEnabled, setLensEnabled] = useState(!!initialDTO.lens);
  const [lens, setLens] = useState(initialDTO.lens ?? "");

  const [tagEnabled, setTagEnabled] = useState(
    !!initialDTO.tag_names && initialDTO.tag_names.length > 0,
  );
  const [tagNames, setTagNames] = useState(initialDTO.tag_names ?? []);

  useEffect(() => {
    if (lastSyncedInitialHashRef.current === initialHash) return;
    lastSyncedInitialHashRef.current = initialHash;

    const next = initialDTO;
    setFilterEnabled(Object.keys(next).length > 0 || hasLockedInitialFilters);

    setTypeEnabled(next.type === "PHOTO" || next.type === "VIDEO");
    setTypeValue(next.type === "VIDEO" ? "VIDEO" : "PHOTO");

    setRawEnabled(typeof next.raw === "boolean");
    setRawMode(next.raw === false ? "exclude" : "include");

    setRatingEnabled(typeof next.rating === "number");
    setRatingValue(typeof next.rating === "number" ? next.rating : 5);

    setLikedEnabled(typeof next.liked === "boolean");
    setLikedValue(next.liked ?? true);

    setFilenameEnabled(!!next.filename);
    setFilenameOperator(next.filename?.operator ?? "contains");
    setFilenameValue(next.filename?.value ?? "");

    setDateEnabled(!!next.date);
    setDateFrom(toDateInput(next.date?.from ?? ""));
    setDateTo(toDateInput(next.date?.to ?? ""));

    setLocationEnabled(!!next.location);
    const nextLocation = next.location ?? EMPTY_LOCATION_BBOX;
    setLocation((previous) =>
      areLocationBBoxesEqual(previous, nextLocation) ? previous : nextLocation,
    );

    setCameraModelEnabled(!!next.camera_model);
    setCameraModel(next.camera_model ?? "");

    setLensEnabled(!!next.lens);
    setLens(next.lens ?? "");

    setTagEnabled(!!next.tag_names && next.tag_names.length > 0);
    setTagNames(next.tag_names ?? []);
  }, [hasLockedInitialFilters, initialDTO, initialHash]);

  const { cameraModelItems, lensItems, loadingOptions } = useFilterOptions({
    open,
    cameraModelOptions,
    lensOptions,
    fetchCameraModels,
    fetchLenses,
  });

  const filterDraft = useMemo<FilterDraft>(
    () => ({
      filterEnabled,
      typeEnabled,
      typeValue,
      rawEnabled,
      rawMode,
      ratingEnabled,
      ratingValue,
      likedEnabled,
      likedValue,
      filenameEnabled,
      filenameOperator,
      filenameValue,
      dateEnabled,
      dateFrom,
      dateTo,
      locationEnabled,
      location,
      cameraModelEnabled,
      cameraModel,
      lensEnabled,
      lens,
      tagEnabled,
      tagNames,
    }),
    [
      filterEnabled,
      typeEnabled,
      typeValue,
      rawEnabled,
      rawMode,
      ratingEnabled,
      ratingValue,
      likedEnabled,
      likedValue,
      filenameEnabled,
      filenameOperator,
      filenameValue,
      dateEnabled,
      dateFrom,
      dateTo,
      locationEnabled,
      location,
      cameraModelEnabled,
      cameraModel,
      lensEnabled,
      lens,
      tagEnabled,
      tagNames,
    ],
  );

  const enabledCount = useMemo(
    () => countEnabledFilters(filterDraft, hasLockedInitialFilters),
    [filterDraft, hasLockedInitialFilters],
  );
  const filterDTO = useMemo(
    () => buildFilterDTO(filterDraft, initialDTO, lockedFieldSet, hasLockedInitialFilters),
    [
      filterDraft,
      hasLockedInitialFilters,
      initialDTO,
      initialHash,
      lockedFieldSet,
      lockedFieldsHash,
    ],
  );

  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    if (!autoApply) return;
    const nextHash = JSON.stringify(filterDTO);
    if (lastAutoAppliedHashRef.current === nextHash) return;
    lastAutoAppliedHashRef.current = nextHash;
    onChangeRef.current?.(filterDTO);
  }, [autoApply, filterDTO]);

  useEffect(() => {
    const handler = (event: MouseEvent) => {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(event.target as Node)) {
        setOpen((previous) => (previous ? false : previous));
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const resetAll = useCallback(() => {
    setFilterEnabled(hasLockedInitialFilters);

    setTypeEnabled(isFieldLocked("type") && isFieldActive(initialDTO, "type"));
    setTypeValue(initialDTO.type === "VIDEO" ? "VIDEO" : "PHOTO");

    setRawEnabled(isFieldLocked("raw") && isFieldActive(initialDTO, "raw"));
    setRawMode(initialDTO.raw === false ? "exclude" : "include");

    setRatingEnabled(isFieldLocked("rating") && isFieldActive(initialDTO, "rating"));
    setRatingValue(typeof initialDTO.rating === "number" ? initialDTO.rating : 5);

    setLikedEnabled(isFieldLocked("liked") && isFieldActive(initialDTO, "liked"));
    setLikedValue(initialDTO.liked ?? true);

    setFilenameEnabled(isFieldLocked("filename") && isFieldActive(initialDTO, "filename"));
    setFilenameOperator(initialDTO.filename?.operator ?? "contains");
    setFilenameValue(initialDTO.filename?.value ?? "");

    setDateEnabled(isFieldLocked("date") && isFieldActive(initialDTO, "date"));
    setDateFrom(toDateInput(initialDTO.date?.from ?? ""));
    setDateTo(toDateInput(initialDTO.date?.to ?? ""));

    setLocationEnabled(isFieldLocked("location") && isFieldActive(initialDTO, "location"));
    setLocation(initialDTO.location ?? EMPTY_LOCATION_BBOX);

    setCameraModelEnabled(
      isFieldLocked("camera_model") && isFieldActive(initialDTO, "camera_model"),
    );
    setCameraModel(initialDTO.camera_model ?? "");

    setLensEnabled(isFieldLocked("lens") && isFieldActive(initialDTO, "lens"));
    setLens(initialDTO.lens ?? "");

    setTagEnabled(isFieldLocked("tag_names") && isFieldActive(initialDTO, "tag_names"));
    setTagNames(initialDTO.tag_names ?? []);

    if (!autoApply) onChange?.(buildLockedInitialDTO(initialDTO, lockedFieldSet));
  }, [
    autoApply,
    hasLockedInitialFilters,
    initialDTO,
    initialHash,
    isFieldLocked,
    lockedFieldSet,
    lockedFieldsHash,
    onChange,
  ]);

  const applyNow = useCallback(() => {
    onChangeRef.current?.(filterDTO);
    setOpen(false);
  }, [filterDTO]);

  const filtersEnabled = filterEnabled || hasLockedInitialFilters;
  const typeLocked = isFieldLocked("type");
  const rawLocked = isFieldLocked("raw");
  const ratingLocked = isFieldLocked("rating");
  const likedLocked = isFieldLocked("liked");
  const filenameLocked = isFieldLocked("filename");
  const dateLocked = isFieldLocked("date");
  const locationLocked = isFieldLocked("location");
  const cameraModelLocked = isFieldLocked("camera_model");
  const lensLocked = isFieldLocked("lens");
  const tagLocked = isFieldLocked("tag_names");

  return (
    <div className={`dropdown dropdown-end ${open ? "dropdown-open" : ""}`} ref={rootRef}>
      <button
        type="button"
        className={`btn btn-sm btn-circle btn-soft btn-info ${filtersEnabled ? "btn-active" : ""} relative`}
        aria-pressed={filtersEnabled}
        onClick={() => setOpen((value) => !value)}
        title={t("assets.filterTool.main.filters_button_title")}
      >
        <ListFilterIcon className="w-4 h-4" />
        {enabledCount > 0 && (
          <span className="badge badge-xs badge-primary absolute -right-1 -top-1 border-base-100">
            {enabledCount}
          </span>
        )}
      </button>

      {open && (
        <div className="dropdown-content bg-base-100 rounded-box z-50 w-80 max-w-[calc(100vw-2rem)] p-0 shadow-xl border border-base-200 mt-2 overflow-hidden flex flex-col">
          <div className="p-4 border-b border-base-200 bg-base-100 sticky top-0 z-10">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <span className="font-bold">{t("assets.filterTool.main.filters_header")}</span>
                {enabledCount > 0 && (
                  <span className="badge badge-primary badge-sm">
                    {t("assets.filterTool.main.active_filters_count", {
                      count: enabledCount,
                    })}
                  </span>
                )}
              </div>
              <label className="label cursor-pointer p-0 gap-2">
                <span className="label-text text-xs opacity-70">
                  {t("assets.filterTool.main.enable_toggle")}
                </span>
                <input
                  type="checkbox"
                  className="toggle toggle-primary toggle-sm"
                  checked={filtersEnabled}
                  disabled={hasLockedInitialFilters}
                  onChange={(event) => {
                    if (!hasLockedInitialFilters) setFilterEnabled(event.target.checked);
                  }}
                />
              </label>
            </div>
          </div>

          <div className="p-4 max-h-[60vh] overflow-y-auto custom-scrollbar">
            <TypeSection
              filterDisabled={!filtersEnabled || typeLocked}
              enabled={typeEnabled}
              onEnabledChange={setTypeEnabled}
              value={typeValue}
              onValueChange={setTypeValue}
            />
            <RawSection
              filterDisabled={!filtersEnabled || rawLocked}
              enabled={rawEnabled}
              onEnabledChange={setRawEnabled}
              mode={rawMode}
              onModeChange={setRawMode}
            />
            <RatingSection
              filterDisabled={!filtersEnabled || ratingLocked}
              enabled={ratingEnabled}
              onEnabledChange={setRatingEnabled}
              value={ratingValue}
              onValueChange={setRatingValue}
            />
            <LikeSection
              filterDisabled={!filtersEnabled || likedLocked}
              enabled={likedEnabled}
              onEnabledChange={setLikedEnabled}
              value={likedValue}
              onValueChange={setLikedValue}
            />
            <FilenameSection
              filterDisabled={!filtersEnabled || filenameLocked}
              enabled={filenameEnabled}
              onEnabledChange={setFilenameEnabled}
              operator={filenameOperator}
              onOperatorChange={setFilenameOperator}
              value={filenameValue}
              onValueChange={setFilenameValue}
            />
            <DateSection
              filterDisabled={!filtersEnabled || dateLocked}
              enabled={dateEnabled}
              onEnabledChange={setDateEnabled}
              from={dateFrom}
              onFromChange={setDateFrom}
              to={dateTo}
              onToChange={setDateTo}
            />
            <LocationSection
              filterDisabled={!filtersEnabled || locationLocked}
              enabled={locationEnabled}
              onEnabledChange={setLocationEnabled}
              bbox={location}
              onBBoxChange={setLocation}
            />
            <CameraMakeSection
              filterDisabled={!filtersEnabled || cameraModelLocked}
              enabled={cameraModelEnabled}
              onEnabledChange={setCameraModelEnabled}
              value={cameraModel}
              onValueChange={setCameraModel}
              items={cameraModelItems}
              loading={loadingOptions}
            />
            <LensSection
              filterDisabled={!filtersEnabled || lensLocked}
              enabled={lensEnabled}
              onEnabledChange={setLensEnabled}
              value={lens}
              onValueChange={setLens}
              items={lensItems}
              loading={loadingOptions}
            />
            <TagSection
              filterDisabled={!filtersEnabled || tagLocked}
              enabled={tagEnabled}
              onEnabledChange={setTagEnabled}
              value={tagNames}
              onValueChange={setTagNames}
            />
          </div>

          <div className="p-3 border-t border-base-200 bg-base-50 sticky bottom-0 z-10 flex items-center justify-between">
            <button
              type="button"
              className="btn btn-xs btn-ghost text-error"
              onClick={resetAll}
              disabled={!filtersEnabled && enabledCount === 0}
            >
              {t("assets.filterTool.main.reset_button")}
            </button>
            {!autoApply && (
              <button type="button" className="btn btn-sm btn-primary" onClick={applyNow}>
                {t("assets.filterTool.main.apply_button")}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
