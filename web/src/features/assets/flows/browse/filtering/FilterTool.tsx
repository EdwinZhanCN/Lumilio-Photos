import { ListFilterIcon } from "lucide-react";
import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n";
import type { AssetUserFilterKey } from "../../../model/filter";
import {
  buildAssetUserFilter,
  buildLockedInitialFilter,
  countEnabledFilters,
  createFilterDraft,
  createLockedFieldSet,
  filterDraftReducer,
  hasActiveLockedFields,
} from "./filterState";
import { LikeSection, RatingSection, RawSection, TypeSection } from "./sections/ChoiceSections";
import { LocationSection } from "./sections/LocationSection";
import { CameraMakeSection, LensSection, TagSection } from "./sections/MetadataSections";
import { DateSection, FilenameSection } from "./sections/ValueSections";
import type { FilterDraft, FilterToolProps } from "./types";
import { useFilterOptions } from "./useFilterOptions";

export type { FilenameOperator, MediaTypeFilter } from "./types";

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
  const initialFilter = useMemo(() => initial ?? {}, [initial]);
  const initialHash = useMemo(() => JSON.stringify(initialFilter), [initialFilter]);
  const lastSyncedInitialHashRef = useRef(initialHash);
  const lastAutoAppliedHashRef = useRef("");

  const lockedFieldSet = useMemo(() => createLockedFieldSet(lockedFields), [lockedFields]);
  const lockedFieldsHash = useMemo(
    () => Array.from(lockedFieldSet).sort().join("|"),
    [lockedFieldSet],
  );
  const isFieldLocked = useCallback(
    (field: AssetUserFilterKey) => lockedFieldSet.has(field),
    [lockedFieldSet],
  );
  const hasLockedInitialFilters = useMemo(
    () => hasActiveLockedFields(initialFilter, lockedFieldSet),
    [initialFilter, initialHash, lockedFieldSet, lockedFieldsHash],
  );
  const [filterDraft, dispatchFilterDraft] = useReducer(
    filterDraftReducer,
    createFilterDraft(initialFilter, hasLockedInitialFilters),
  );
  const setDraftField = useCallback(
    <Key extends keyof FilterDraft>(key: Key, value: FilterDraft[Key]) => {
      dispatchFilterDraft({ type: "set", key, value });
    },
    [],
  );

  useEffect(() => {
    if (lastSyncedInitialHashRef.current === initialHash) return;
    lastSyncedInitialHashRef.current = initialHash;

    dispatchFilterDraft({
      type: "replace",
      draft: createFilterDraft(initialFilter, hasLockedInitialFilters),
    });
  }, [hasLockedInitialFilters, initialFilter, initialHash]);

  const { cameraModelItems, lensItems, loadingOptions } = useFilterOptions({
    open,
    cameraModelOptions,
    lensOptions,
    fetchCameraModels,
    fetchLenses,
  });

  const enabledCount = useMemo(
    () => countEnabledFilters(filterDraft, hasLockedInitialFilters),
    [filterDraft, hasLockedInitialFilters],
  );
  const appliedFilter = useMemo(
    () => buildAssetUserFilter(filterDraft, initialFilter, lockedFieldSet, hasLockedInitialFilters),
    [
      filterDraft,
      hasLockedInitialFilters,
      initialFilter,
      initialHash,
      lockedFieldSet,
      lockedFieldsHash,
    ],
  );

  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  useEffect(() => {
    if (!autoApply) return;
    const nextHash = JSON.stringify(appliedFilter);
    if (lastAutoAppliedHashRef.current === nextHash) return;
    lastAutoAppliedHashRef.current = nextHash;
    onChangeRef.current?.(appliedFilter);
  }, [appliedFilter, autoApply]);

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
    const lockedFilter = buildLockedInitialFilter(initialFilter, lockedFieldSet);
    dispatchFilterDraft({
      type: "replace",
      draft: createFilterDraft(lockedFilter, hasLockedInitialFilters),
    });
    if (!autoApply) onChange?.(lockedFilter);
  }, [
    autoApply,
    hasLockedInitialFilters,
    initialFilter,
    initialHash,
    lockedFieldSet,
    lockedFieldsHash,
    onChange,
  ]);

  const applyNow = useCallback(() => {
    onChangeRef.current?.(appliedFilter);
    setOpen(false);
  }, [appliedFilter]);

  const filtersEnabled = filterDraft.filterEnabled || hasLockedInitialFilters;
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
                    if (!hasLockedInitialFilters) {
                      setDraftField("filterEnabled", event.target.checked);
                    }
                  }}
                />
              </label>
            </div>
          </div>

          <div className="p-4 max-h-[60vh] overflow-y-auto custom-scrollbar">
            <TypeSection
              filterDisabled={!filtersEnabled || typeLocked}
              enabled={filterDraft.typeEnabled}
              onEnabledChange={(value) => setDraftField("typeEnabled", value)}
              value={filterDraft.typeValue}
              onValueChange={(value) => setDraftField("typeValue", value)}
            />
            <RawSection
              filterDisabled={!filtersEnabled || rawLocked}
              enabled={filterDraft.rawEnabled}
              onEnabledChange={(value) => setDraftField("rawEnabled", value)}
              mode={filterDraft.rawMode}
              onModeChange={(value) => setDraftField("rawMode", value)}
            />
            <RatingSection
              filterDisabled={!filtersEnabled || ratingLocked}
              enabled={filterDraft.ratingEnabled}
              onEnabledChange={(value) => setDraftField("ratingEnabled", value)}
              value={filterDraft.ratingValue}
              onValueChange={(value) => setDraftField("ratingValue", value)}
            />
            <LikeSection
              filterDisabled={!filtersEnabled || likedLocked}
              enabled={filterDraft.likedEnabled}
              onEnabledChange={(value) => setDraftField("likedEnabled", value)}
              value={filterDraft.likedValue}
              onValueChange={(value) => setDraftField("likedValue", value)}
            />
            <FilenameSection
              filterDisabled={!filtersEnabled || filenameLocked}
              enabled={filterDraft.filenameEnabled}
              onEnabledChange={(value) => setDraftField("filenameEnabled", value)}
              operator={filterDraft.filenameOperator}
              onOperatorChange={(value) => setDraftField("filenameOperator", value)}
              value={filterDraft.filenameValue}
              onValueChange={(value) => setDraftField("filenameValue", value)}
            />
            <DateSection
              filterDisabled={!filtersEnabled || dateLocked}
              enabled={filterDraft.dateEnabled}
              onEnabledChange={(value) => setDraftField("dateEnabled", value)}
              from={filterDraft.dateFrom}
              onFromChange={(value) => setDraftField("dateFrom", value)}
              to={filterDraft.dateTo}
              onToChange={(value) => setDraftField("dateTo", value)}
            />
            <LocationSection
              filterDisabled={!filtersEnabled || locationLocked}
              enabled={filterDraft.locationEnabled}
              onEnabledChange={(value) => setDraftField("locationEnabled", value)}
              bbox={filterDraft.location}
              onBBoxChange={(value) => setDraftField("location", value)}
            />
            <CameraMakeSection
              filterDisabled={!filtersEnabled || cameraModelLocked}
              enabled={filterDraft.cameraModelEnabled}
              onEnabledChange={(value) => setDraftField("cameraModelEnabled", value)}
              value={filterDraft.cameraModel}
              onValueChange={(value) => setDraftField("cameraModel", value)}
              items={cameraModelItems}
              loading={loadingOptions}
            />
            <LensSection
              filterDisabled={!filtersEnabled || lensLocked}
              enabled={filterDraft.lensEnabled}
              onEnabledChange={(value) => setDraftField("lensEnabled", value)}
              value={filterDraft.lens}
              onValueChange={(value) => setDraftField("lens", value)}
              items={lensItems}
              loading={loadingOptions}
            />
            <TagSection
              filterDisabled={!filtersEnabled || tagLocked}
              enabled={filterDraft.tagEnabled}
              onEnabledChange={(value) => setDraftField("tagEnabled", value)}
              value={filterDraft.tagNames}
              onValueChange={(value) => setDraftField("tagNames", value)}
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
