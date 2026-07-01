import { ListFilterIcon } from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { useI18n } from "@/lib/i18n";
import MapComponent from "@/components/MapComponent";
import TagPickerMenu, {
  type TagPickerItem,
} from "@/features/assets/components/shared/TagPickerMenu";

type TagOption = components["schemas"]["dto.TagDTO"];

/* =========================
   Types
   ========================= */

type FilenameOperator = "contains" | "matches" | "starts_with" | "ends_with";
type MediaTypeFilter = "PHOTO" | "VIDEO";

export interface FilenameFilter {
  operator: FilenameOperator;
  value: string;
}

export interface DateRange {
  from?: string; // YYYY-MM-DD
  to?: string; // YYYY-MM-DD
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
  rating?: number; // 0-5, where 0 means unrated
  liked?: boolean;
  filename?: FilenameFilter;
  date?: DateRange;
  camera_model?: string;
  lens?: string;
  tag_names?: string[];

  // Extended field to represent spatial filtering
  location?: LocationBBox;
}

export type FilterFieldKey = keyof FilterDTO;

type FilterToolProps = {
  initial?: FilterDTO;
  onChange?: (filters: FilterDTO) => void;
  autoApply?: boolean;
  lockedFields?: readonly FilterFieldKey[] | Partial<Record<FilterFieldKey, boolean>>;

  // Options can be provided directly via props, or fetched via provided functions, or fetched from default endpoints.
  cameraModelOptions?: string[];
  lensOptions?: string[];
  fetchCameraModels?: () => Promise<string[]>;
  fetchLenses?: () => Promise<string[]>;
};

const EMPTY_LOCATION_BBOX: LocationBBox = {
  north: 0,
  south: 0,
  east: 0,
  west: 0,
};

/* =========================
   Pure helpers
   ========================= */

function centerToBBox(lat: number, lon: number, radiusKm: number): LocationBBox {
  const dLat = radiusKm / 110.574; // degrees
  const dLon = radiusKm / (111.32 * Math.cos((lat * Math.PI) / 180));
  return {
    north: lat + dLat,
    south: lat - dLat,
    east: lon + dLon,
    west: lon - dLon,
  };
}

function isZeroBBox(b: LocationBBox): boolean {
  return b.north === 0 && b.south === 0 && b.east === 0 && b.west === 0;
}

function areLocationBBoxesEqual(left: LocationBBox, right: LocationBBox): boolean {
  return (
    left.north === right.north &&
    left.south === right.south &&
    left.east === right.east &&
    left.west === right.west
  );
}

function toDateInput(val: string): string {
  if (!val) return "";
  // If it contains 'T', assume ISO format and take the date part
  if (val.includes("T")) {
    return val.split("T")[0];
  }
  return val;
}

function isFieldActive(dto: FilterDTO, field: FilterFieldKey): boolean {
  switch (field) {
    case "type":
      return dto.type === "PHOTO" || dto.type === "VIDEO";
    case "raw":
      return typeof dto.raw === "boolean";
    case "rating":
      return typeof dto.rating === "number";
    case "liked":
      return typeof dto.liked === "boolean";
    case "filename":
      return !!dto.filename?.value?.trim();
    case "date":
      return !!dto.date && (!!dto.date.from || !!dto.date.to);
    case "camera_model":
      return !!dto.camera_model?.trim();
    case "lens":
      return !!dto.lens?.trim();
    case "tag_names":
      return !!dto.tag_names && dto.tag_names.length > 0;
    case "location":
      return !!dto.location && !isZeroBBox(dto.location);
  }
}

function buildLockedInitialDTO(
  initial: FilterDTO,
  lockedFieldSet: ReadonlySet<FilterFieldKey>,
): FilterDTO {
  const dto: FilterDTO = {};

  if (lockedFieldSet.has("type") && isFieldActive(initial, "type")) {
    dto.type = initial.type;
  }
  if (lockedFieldSet.has("raw") && isFieldActive(initial, "raw")) {
    dto.raw = initial.raw;
  }
  if (lockedFieldSet.has("rating") && isFieldActive(initial, "rating")) {
    dto.rating = initial.rating;
  }
  if (lockedFieldSet.has("liked") && isFieldActive(initial, "liked")) {
    dto.liked = initial.liked;
  }
  if (lockedFieldSet.has("filename") && isFieldActive(initial, "filename")) {
    dto.filename = {
      operator: initial.filename!.operator,
      value: initial.filename!.value.trim(),
    };
  }
  if (lockedFieldSet.has("date") && isFieldActive(initial, "date")) {
    dto.date = {
      from: initial.date!.from,
      to: initial.date!.to,
    };
  }
  if (lockedFieldSet.has("camera_model") && isFieldActive(initial, "camera_model")) {
    dto.camera_model = initial.camera_model!.trim();
  }
  if (lockedFieldSet.has("lens") && isFieldActive(initial, "lens")) {
    dto.lens = initial.lens!.trim();
  }
  if (lockedFieldSet.has("tag_names") && isFieldActive(initial, "tag_names")) {
    dto.tag_names = [...initial.tag_names!];
  }
  if (lockedFieldSet.has("location") && isFieldActive(initial, "location")) {
    dto.location = { ...initial.location! };
  }

  return dto;
}

function mergeLockedInitialDTO(
  dto: FilterDTO,
  initial: FilterDTO,
  lockedFieldSet: ReadonlySet<FilterFieldKey>,
): FilterDTO {
  return {
    ...dto,
    ...buildLockedInitialDTO(initial, lockedFieldSet),
  };
}

/* =========================
   Small hook: options loading
   ========================= */

function useFilterOptions({
  open,
  cameraModelOptions,
  lensOptions,
  fetchCameraModels,
  fetchLenses,
}: {
  open: boolean;
  cameraModelOptions?: string[];
  lensOptions?: string[];
  fetchCameraModels?: () => Promise<string[]>;
  fetchLenses?: () => Promise<string[]>;
}) {
  const [cameraModelItems, setCameraModelItems] = useState<string[]>(cameraModelOptions ?? []);
  const [lensItems, setLensItems] = useState<string[]>(lensOptions ?? []);
  const [isCustomLoading, setIsCustomLoading] = useState<boolean>(false);
  const [hasLoaded, setHasLoaded] = useState(false);

  const needsCameraModels = !cameraModelOptions || cameraModelOptions.length === 0;
  const needsLenses = !lensOptions || lensOptions.length === 0;
  const needsOptions = needsCameraModels || needsLenses;
  const canUseCustomFetchers = !!fetchCameraModels && !!fetchLenses;
  const shouldFetchDefault = open && !hasLoaded && needsOptions && !canUseCustomFetchers;

  const filterOptionsQuery = $api.useQuery(
    "get",
    "/api/v1/assets/filter-options",
    {},
    {
      enabled: shouldFetchDefault,
    },
  );
  const loadingOptions = isCustomLoading || filterOptionsQuery.isFetching;

  useEffect(() => {
    const shouldFetch = open && !hasLoaded && needsOptions && canUseCustomFetchers;
    if (!shouldFetch) return;

    let running = true;
    const load = async () => {
      try {
        setIsCustomLoading(true);

        let cm: string[] = cameraModelOptions ?? [];
        let ln: string[] = lensOptions ?? [];

        cm = await fetchCameraModels!();
        ln = await fetchLenses!();

        if (running) {
          setCameraModelItems(cm);
          setLensItems(ln);
          setHasLoaded(true);
        }
      } catch {
        // ignore
      } finally {
        if (running) setIsCustomLoading(false);
      }
    };
    void load();

    return () => {
      running = false;
    };
  }, [
    open,
    cameraModelOptions,
    lensOptions,
    fetchCameraModels,
    fetchLenses,
    needsOptions,
    canUseCustomFetchers,
    hasLoaded,
  ]);

  useEffect(() => {
    if (!shouldFetchDefault) return;
    const response = filterOptionsQuery.data;
    if (!response) return;

    if (needsCameraModels) {
      setCameraModelItems(response.camera_models || []);
    }
    if (needsLenses) {
      setLensItems(response.lenses || []);
    }

    setHasLoaded(true);
  }, [shouldFetchDefault, filterOptionsQuery.data, needsCameraModels, needsLenses]);

  return { cameraModelItems, lensItems, loadingOptions };
}

/* =========================
   Reusable section shell
   ========================= */

const SectionShell = memo(function SectionShell({
  title,
  enabled,
  onToggle,
  disabled,
  children,
}: {
  title: string;
  enabled: boolean;
  onToggle: (v: boolean) => void;
  disabled?: boolean;
  children: React.ReactNode;
}) {
  const { t } = useI18n();
  return (
    <div className="form-control mb-3">
      <div className="flex items-center justify-between">
        <span className="label-text font-medium">{title}</span>
        <label className="label cursor-pointer p-0 gap-2">
          <span className="label-text">{t("assets.filterTool.sectionShell.enable")}</span>
          <input
            type="checkbox"
            className="toggle toggle-primary toggle-sm"
            disabled={disabled}
            checked={enabled}
            onChange={(e) => onToggle(e.target.checked)}
          />
        </label>
      </div>
      <div className="mt-2">{children}</div>
    </div>
  );
});

/* =========================
   Business sections
   ========================= */

const RawSection = memo(function RawSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  mode,
  onModeChange,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  mode: "include" | "exclude";
  onModeChange: (m: "include" | "exclude") => void;
}) {
  const { t } = useI18n();
  return (
    <SectionShell
      title={t("assets.filterTool.rawSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join w-full">
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${mode === "include" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onModeChange("include")}
        >
          {t("assets.filterTool.rawSection.include")}
        </button>
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${mode === "exclude" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onModeChange("exclude")}
        >
          {t("assets.filterTool.rawSection.exclude")}
        </button>
      </div>
    </SectionShell>
  );
});

const TypeSection = memo(function TypeSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  value: MediaTypeFilter;
  onValueChange: (value: MediaTypeFilter) => void;
}) {
  const { t } = useI18n();
  return (
    <SectionShell
      title={t("assets.filterTool.typeSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join w-full">
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${value === "PHOTO" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange("PHOTO")}
        >
          {t("assets.filterTool.typeSection.photo")}
        </button>
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${value === "VIDEO" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange("VIDEO")}
        >
          {t("assets.filterTool.typeSection.video")}
        </button>
      </div>
    </SectionShell>
  );
});

const RatingSection = memo(function RatingSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  value: number;
  onValueChange: (n: number) => void;
}) {
  const { t } = useI18n();
  return (
    <SectionShell
      title={t("assets.filterTool.ratingSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join w-full flex">
        {[5, 4, 3, 2, 1].map((n) => (
          <button
            key={n}
            type="button"
            className={`btn btn-xs join-item flex-1 ${value === n ? "btn-primary" : "btn-outline"}`}
            disabled={filterDisabled || !enabled}
            onClick={() => onValueChange(n)}
            title={t("assets.filterTool.ratingSection.rating_n", { n })}
          >
            {n}
          </button>
        ))}
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${value === 0 ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(0)}
          title={t("assets.filterTool.ratingSection.unrated_title")}
        >
          {t("assets.filterTool.ratingSection.unrated_short")}
        </button>
      </div>
    </SectionShell>
  );
});

const LikeSection = memo(function LikeSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  value: boolean;
  onValueChange: (v: boolean) => void;
}) {
  const { t } = useI18n();
  return (
    <SectionShell
      title={t("assets.filterTool.likeSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join w-full">
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${value ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(true)}
        >
          {t("assets.filterTool.likeSection.liked")}
        </button>
        <button
          type="button"
          className={`btn btn-xs join-item flex-1 ${!value ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(false)}
        >
          {t("assets.filterTool.likeSection.unliked")}
        </button>
      </div>
    </SectionShell>
  );
});

const FilenameSection = memo(function FilenameSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  operator,
  onOperatorChange,
  value,
  onValueChange,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  operator: FilenameOperator;
  onOperatorChange: (op: FilenameOperator) => void;
  value: string;
  onValueChange: (v: string) => void;
}) {
  const { t } = useI18n();
  return (
    <SectionShell
      title={t("assets.filterTool.filenameSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="flex flex-col gap-2">
        <select
          className="select select-bordered select-xs w-full"
          disabled={filterDisabled || !enabled}
          value={operator}
          onChange={(e) => onOperatorChange(e.target.value as FilenameOperator)}
        >
          <option value="contains">{t("assets.filterTool.filenameSection.contains")}</option>
          <option value="matches">{t("assets.filterTool.filenameSection.matches")}</option>
          <option value="starts_with">{t("assets.filterTool.filenameSection.starts_with")}</option>
          <option value="ends_with">{t("assets.filterTool.filenameSection.ends_with")}</option>
        </select>
        <input
          type="text"
          className="input input-bordered input-xs w-full"
          placeholder={t("assets.filterTool.filenameSection.placeholder")}
          disabled={filterDisabled || !enabled}
          value={value}
          onChange={(e) => onValueChange(e.target.value)}
        />
      </div>
    </SectionShell>
  );
});

const DateSection = memo(function DateSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  from,
  onFromChange,
  to,
  onToChange,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  from: string;
  onFromChange: (v: string) => void;
  to: string;
  onToChange: (v: string) => void;
}) {
  const { t } = useI18n();
  return (
    <SectionShell
      title={t("assets.filterTool.dateSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="flex flex-col gap-2">
        <label className="input input-bordered input-xs w-full flex items-center gap-2">
          <span className="text-xs opacity-70 w-8">{t("assets.filterTool.dateSection.from")}</span>
          <input
            type="date"
            className="grow text-xs"
            value={from}
            disabled={filterDisabled || !enabled}
            onChange={(e) => onFromChange(e.target.value)}
          />
        </label>
        <label className="input input-bordered input-xs w-full flex items-center gap-2">
          <span className="text-xs opacity-70 w-8">{t("assets.filterTool.dateSection.to")}</span>
          <input
            type="date"
            className="grow text-xs"
            value={to}
            disabled={filterDisabled || !enabled}
            onChange={(e) => onToChange(e.target.value)}
          />
        </label>
      </div>
    </SectionShell>
  );
});

const LocationSection = memo(function LocationSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  bbox,
  onBBoxChange,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  bbox: LocationBBox;
  onBBoxChange: (b: LocationBBox) => void;
}) {
  const [mapModalOpen, setMapModalOpen] = useState<boolean>(false);
  const [locationRadiusKm, setLocationRadiusKm] = useState<number>(5);
  const [locationCenterLat, setLocationCenterLat] = useState<number>(0);
  const [locationCenterLon, setLocationCenterLon] = useState<number>(0);
  const { t } = useI18n();

  const setCurrentLocationAsCenter = useCallback(() => {
    if (!navigator.geolocation) return;
    navigator.geolocation.getCurrentPosition((pos) => {
      const { latitude, longitude } = pos.coords;
      setLocationCenterLat(latitude);
      setLocationCenterLon(longitude);
    });
  }, []);

  const computeBBoxFromCenter = useCallback(() => {
    const newBBox = centerToBBox(locationCenterLat, locationCenterLon, locationRadiusKm);
    onBBoxChange(newBBox);
  }, [locationCenterLat, locationCenterLon, locationRadiusKm, onBBoxChange]);

  const previewCenter = useMemo<[number, number]>(() => {
    if (!isZeroBBox(bbox)) {
      return [(bbox.north + bbox.south) / 2, (bbox.east + bbox.west) / 2];
    }
    return [locationCenterLat, locationCenterLon];
  }, [bbox, locationCenterLat, locationCenterLon]);

  const previewBounds = isZeroBBox(bbox) ? undefined : bbox;

  return (
    <>
      <SectionShell
        title={t("assets.filterTool.locationSection.title")}
        enabled={enabled}
        onToggle={onEnabledChange}
        disabled={filterDisabled}
      >
        <div className="flex flex-col gap-2">
          <div className="flex gap-2">
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder={t("assets.filterTool.locationSection.north_placeholder")}
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.north}
              onChange={(e) => onBBoxChange({ ...bbox, north: Number(e.target.value) })}
            />
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder={t("assets.filterTool.locationSection.south_placeholder")}
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.south}
              onChange={(e) => onBBoxChange({ ...bbox, south: Number(e.target.value) })}
            />
          </div>
          <div className="flex gap-2">
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder={t("assets.filterTool.locationSection.east_placeholder")}
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.east}
              onChange={(e) => onBBoxChange({ ...bbox, east: Number(e.target.value) })}
            />
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder={t("assets.filterTool.locationSection.west_placeholder")}
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.west}
              onChange={(e) => onBBoxChange({ ...bbox, west: Number(e.target.value) })}
            />
          </div>

          <div className="flex items-center gap-2 mt-1">
            <button
              type="button"
              className="btn btn-xs btn-outline flex-1"
              disabled={filterDisabled || !enabled}
              onClick={() => setMapModalOpen(true)}
            >
              {t("assets.filterTool.locationSection.pick_on_map")}
            </button>
            <button
              type="button"
              className="btn btn-xs btn-ghost flex-1"
              disabled={filterDisabled || !enabled}
              onClick={setCurrentLocationAsCenter}
            >
              {t("assets.filterTool.locationSection.use_current_location")}
            </button>
          </div>
        </div>
      </SectionShell>

      {mapModalOpen && (
        <dialog className="modal modal-open">
          <div className="modal-box max-w-sm">
            <h3 className="font-bold text-lg">
              {t("assets.filterTool.locationSection.modal_title")}
            </h3>
            <p className="py-2 text-sm opacity-80">
              {t("assets.filterTool.locationSection.modal_description")}
            </p>

            <div className="grid grid-cols-2 gap-2 mt-2">
              <label className="form-control">
                <span className="label-text">
                  {t("assets.filterTool.locationSection.center_lat")}
                </span>
                <input
                  type="number"
                  className="input input-bordered input-sm"
                  step="0.000001"
                  value={locationCenterLat}
                  onChange={(e) => setLocationCenterLat(Number(e.target.value))}
                />
              </label>

              <label className="form-control">
                <span className="label-text">
                  {t("assets.filterTool.locationSection.center_lon")}
                </span>
                <input
                  type="number"
                  className="input input-bordered input-sm"
                  step="0.000001"
                  value={locationCenterLon}
                  onChange={(e) => setLocationCenterLon(Number(e.target.value))}
                />
              </label>

              <label className="form-control col-span-2">
                <span className="label-text">
                  {t("assets.filterTool.locationSection.radius_km")}
                </span>
                <input
                  type="number"
                  className="input input-bordered input-sm"
                  min={0.1}
                  step={0.1}
                  value={locationRadiusKm}
                  onChange={(e) => setLocationRadiusKm(Number(e.target.value))}
                />
              </label>
            </div>

            <div className="mt-3 flex items-center gap-2">
              <button
                type="button"
                className="btn btn-sm btn-outline flex-1"
                onClick={setCurrentLocationAsCenter}
              >
                {t("assets.filterTool.locationSection.use_current_location")}
              </button>
              <button type="button" className="btn btn-sm flex-1" onClick={computeBBoxFromCenter}>
                {t("assets.filterTool.locationSection.generate_bbox")}
              </button>
            </div>

            <div className="mt-4">
              <div className="text-sm opacity-70 mb-2">
                {t("assets.filterTool.locationSection.preview_map")}
              </div>
              <div className="w-full h-40 rounded-box overflow-hidden border border-base-300">
                <MapComponent
                  center={previewCenter}
                  zoom={previewBounds ? 10 : 3}
                  height="100%"
                  rounded={false}
                  boundsOverlay={previewBounds}
                />
              </div>
              <div className="text-xs opacity-70 mt-2">
                {t("assets.filterTool.locationSection.bbox_coords", {
                  north: bbox.north.toFixed(6),
                  south: bbox.south.toFixed(6),
                  east: bbox.east.toFixed(6),
                  west: bbox.west.toFixed(6),
                })}
              </div>
            </div>

            <div className="modal-action">
              <button
                type="button"
                className="btn btn-ghost btn-sm"
                onClick={() => setMapModalOpen(false)}
              >
                {t("assets.filterTool.locationSection.cancel")}
              </button>
              <button
                type="button"
                className="btn btn-primary btn-sm"
                onClick={() => {
                  setMapModalOpen(false);
                }}
              >
                {t("assets.filterTool.locationSection.done")}
              </button>
            </div>
          </div>
          <form method="dialog" className="modal-backdrop" onClick={() => setMapModalOpen(false)}>
            <button>{t("assets.filterTool.locationSection.close_modal")}</button>
          </form>
        </dialog>
      )}
    </>
  );
});

const CameraMakeSection = memo(function CameraMakeSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
  items,
  loading,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  value: string;
  onValueChange: (v: string) => void;
  items: string[];
  loading: boolean;
}) {
  const { t } = useI18n();
  return (
    <SectionShell
      title={t("assets.filterTool.cameraMakeSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <select
        className="select select-bordered select-xs w-full"
        disabled={filterDisabled || !enabled || loading}
        value={value}
        onChange={(e) => onValueChange(e.target.value)}
      >
        <option value="">{t("assets.filterTool.cameraMakeSection.select_placeholder")}</option>
        {items.map((item) => (
          <option key={item} value={item}>
            {item}
          </option>
        ))}
      </select>
      {loading && (
        <span className="text-xs opacity-70 mt-1 block">
          {t("assets.filterTool.cameraMakeSection.loading_options")}
        </span>
      )}
    </SectionShell>
  );
});

const LensSection = memo(function LensSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
  items,
  loading,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  value: string;
  onValueChange: (v: string) => void;
  items: string[];
  loading: boolean;
}) {
  const { t } = useI18n();
  return (
    <SectionShell
      title={t("assets.filterTool.lensSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <select
        className="select select-bordered select-xs w-full"
        disabled={filterDisabled || !enabled || loading}
        value={value}
        onChange={(e) => onValueChange(e.target.value)}
      >
        <option value="">{t("assets.filterTool.lensSection.select_placeholder")}</option>
        {items.map((item) => (
          <option key={item} value={item}>
            {item}
          </option>
        ))}
      </select>
      {loading && (
        <span className="text-xs opacity-70 mt-1 block">
          {t("assets.filterTool.lensSection.loading_options")}
        </span>
      )}
    </SectionShell>
  );
});

const TagSection = memo(function TagSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  value,
  onValueChange,
}: {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (v: boolean) => void;
  value: string[];
  onValueChange: (v: string[]) => void;
}) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const active = enabled && !filterDisabled;

  const tagsQuery = $api.useQuery(
    "get",
    "/api/v1/assets/tags",
    { params: { query: { q: query, limit: 20 } } },
    { enabled: active, staleTime: 30_000 },
  );
  const options: TagOption[] = tagsQuery.data?.tags ?? [];
  const selected = new Set(value);
  const suggestions: TagPickerItem[] = options
    .filter((tag) => tag.tag_name && !selected.has(tag.tag_name))
    .map((tag) => ({ id: tag.tag_id ?? tag.tag_name!, name: tag.tag_name! }));
  const checked: TagPickerItem[] = value.map((name) => ({ id: name, name }));

  return (
    <SectionShell
      title={t("assets.filterTool.tagSection.title")}
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <TagPickerMenu
        query={query}
        onQueryChange={setQuery}
        placeholder={t("assets.filterTool.tagSection.placeholder")}
        loading={tagsQuery.isFetching}
        loadingText={t("assets.filterTool.tagSection.loading")}
        noResultsText={t("assets.filterTool.tagSection.no_results")}
        checked={checked}
        suggestions={active ? suggestions : []}
        onToggleChecked={(item) => onValueChange(value.filter((name) => name !== item.name))}
        onSelectSuggestion={(item) => onValueChange([...value, item.name])}
        className="max-h-52"
      />
    </SectionShell>
  );
});

/* =========================
   Main component
   ========================= */

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
  // Dropdown open state (independent of filter enabled)
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const initialDTO = useMemo(() => initial ?? {}, [initial]);
  const initialHash = useMemo(() => JSON.stringify(initialDTO), [initialDTO]);
  const lastSyncedInitialHashRef = useRef<string>(initialHash);
  const lastAutoAppliedHashRef = useRef<string>("");
  const lockedFieldSet = useMemo(() => {
    if (!lockedFields) return new Set<FilterFieldKey>();
    if (Array.isArray(lockedFields)) {
      return new Set<FilterFieldKey>(lockedFields);
    }
    return new Set<FilterFieldKey>(
      (Object.entries(lockedFields) as [FilterFieldKey, boolean | undefined][])
        .filter(([, locked]) => locked)
        .map(([field]) => field),
    );
  }, [lockedFields]);
  const lockedFieldsHash = useMemo(
    () => Array.from(lockedFieldSet).sort().join("|"),
    [lockedFieldSet],
  );
  const isFieldLocked = useCallback(
    (field: FilterFieldKey) => lockedFieldSet.has(field),
    [lockedFieldSet],
  );
  const hasLockedInitialFilters = useMemo(
    () => Array.from(lockedFieldSet).some((field) => isFieldActive(initialDTO, field)),
    [initialDTO, initialHash, lockedFieldSet, lockedFieldsHash],
  );

  // Global filter enable/disable
  const [filterEnabled, setFilterEnabled] = useState<boolean>(
    Object.keys(initialDTO).length > 0 || hasLockedInitialFilters,
  );

  // Type
  const [typeEnabled, setTypeEnabled] = useState<boolean>(
    initialDTO.type === "PHOTO" || initialDTO.type === "VIDEO",
  );
  const [typeValue, setTypeValue] = useState<MediaTypeFilter>(
    initialDTO.type === "VIDEO" ? "VIDEO" : "PHOTO",
  );

  // RAW
  const [rawEnabled, setRawEnabled] = useState<boolean>(typeof initialDTO.raw === "boolean");
  const [rawMode, setRawMode] = useState<"include" | "exclude">(
    initialDTO.raw === false ? "exclude" : "include",
  );

  // Rating: 5/4/3/2/1/unrated(0)
  const [ratingEnabled, setRatingEnabled] = useState<boolean>(
    typeof initialDTO.rating === "number",
  );
  const [ratingValue, setRatingValue] = useState<number>(
    typeof initialDTO.rating === "number" ? initialDTO.rating : 5,
  );

  // Liked: liked/unliked
  const [likedEnabled, setLikedEnabled] = useState<boolean>(typeof initialDTO.liked === "boolean");
  const [likedValue, setLikedValue] = useState<boolean>(initialDTO.liked ?? true);

  // Filename: operator + value
  const [filenameEnabled, setFilenameEnabled] = useState<boolean>(!!initialDTO.filename);
  const [filenameOperator, setFilenameOperator] = useState<FilenameOperator>(
    initialDTO.filename?.operator ?? "contains",
  );
  const [filenameValue, setFilenameValue] = useState<string>(initialDTO.filename?.value ?? "");

  // Date range
  const [dateEnabled, setDateEnabled] = useState<boolean>(!!initialDTO.date);
  const [dateFrom, setDateFrom] = useState<string>(toDateInput(initialDTO.date?.from ?? ""));
  const [dateTo, setDateTo] = useState<string>(toDateInput(initialDTO.date?.to ?? ""));

  // Location (BBox)
  const [locationEnabled, setLocationEnabled] = useState<boolean>(!!initialDTO.location);
  const [location, setLocation] = useState<LocationBBox>(
    initialDTO.location ?? EMPTY_LOCATION_BBOX,
  );

  // Camera model / Lens
  const [cameraModelEnabled, setCameraModelEnabled] = useState<boolean>(!!initialDTO.camera_model);
  const [cameraModel, setCameraModel] = useState<string>(initialDTO.camera_model ?? "");
  const [lensEnabled, setLensEnabled] = useState<boolean>(!!initialDTO.lens);
  const [lens, setLens] = useState<string>(initialDTO.lens ?? "");

  // Tag (multi-select)
  const [tagEnabled, setTagEnabled] = useState<boolean>(
    !!initialDTO.tag_names && initialDTO.tag_names.length > 0,
  );
  const [tagNames, setTagNames] = useState<string[]>(initialDTO.tag_names ?? []);

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
    setLocation((prev) => (areLocationBBoxesEqual(prev, nextLocation) ? prev : nextLocation));

    setCameraModelEnabled(!!next.camera_model);
    setCameraModel(next.camera_model ?? "");

    setLensEnabled(!!next.lens);
    setLens(next.lens ?? "");

    setTagEnabled(!!next.tag_names && next.tag_names.length > 0);
    setTagNames(next.tag_names ?? []);
  }, [hasLockedInitialFilters, initialDTO, initialHash]);

  // Options hook
  const { cameraModelItems, lensItems, loadingOptions } = useFilterOptions({
    open,
    cameraModelOptions,
    lensOptions,
    fetchCameraModels,
    fetchLenses,
  });

  const enabledCount = useMemo(() => {
    if (!filterEnabled && !hasLockedInitialFilters) return 0;
    let count = 0;
    if (rawEnabled) count++;
    if (typeEnabled) count++;
    if (ratingEnabled) count++;
    if (likedEnabled) count++;
    if (filenameEnabled && filenameValue.trim() !== "") count++;
    if (dateEnabled && (dateFrom || dateTo)) count++;
    if (locationEnabled && !isZeroBBox(location)) count++;
    if (cameraModelEnabled && cameraModel) count++;
    if (lensEnabled && lens) count++;
    if (tagEnabled && tagNames.length > 0) count++;
    return count;
  }, [
    filterEnabled,
    typeEnabled,
    rawEnabled,
    ratingEnabled,
    likedEnabled,
    filenameEnabled,
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
    hasLockedInitialFilters,
  ]);

  // Build DTO from local UI state only when a section is enabled and has valid value.
  // Disabled sections are ignored; for location, ignore zero bounding box to avoid sending empty spatial filters.
  const buildDTO = useCallback((): FilterDTO => {
    if (!filterEnabled && !hasLockedInitialFilters) return {};
    const dto: FilterDTO = {};
    if (filterEnabled && typeEnabled) dto.type = typeValue;
    if (filterEnabled && rawEnabled) dto.raw = rawMode === "include";
    if (filterEnabled && ratingEnabled) dto.rating = ratingValue;
    if (filterEnabled && likedEnabled) dto.liked = likedValue;
    if (filterEnabled && filenameEnabled && filenameValue.trim()) {
      dto.filename = {
        operator: filenameOperator,
        value: filenameValue.trim(),
      };
    }
    if (filterEnabled && dateEnabled && (dateFrom || dateTo)) {
      dto.date = {
        from: dateFrom || undefined,
        to: dateTo || undefined,
      };
    }
    if (filterEnabled && locationEnabled && !isZeroBBox(location)) {
      dto.location = { ...location };
    }
    if (filterEnabled && cameraModelEnabled && cameraModel) {
      dto.camera_model = cameraModel;
    }
    if (filterEnabled && lensEnabled && lens) dto.lens = lens;
    if (filterEnabled && tagEnabled && tagNames.length > 0) {
      dto.tag_names = tagNames;
    }
    return mergeLockedInitialDTO(dto, initialDTO, lockedFieldSet);
  }, [
    filterEnabled,
    hasLockedInitialFilters,
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
    initialDTO,
    initialHash,
    lockedFieldSet,
    lockedFieldsHash,
  ]);

  // Use ref to store the latest onChange callback to avoid dependency issues
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;

  // Memoize the filter DTO to prevent unnecessary re-renders
  const filterDTO = useMemo(() => buildDTO(), [buildDTO]);

  // Auto-emit on filter state change if enabled
  useEffect(() => {
    if (!autoApply) return;
    const nextHash = JSON.stringify(filterDTO);
    if (lastAutoAppliedHashRef.current === nextHash) return;
    lastAutoAppliedHashRef.current = nextHash;
    onChangeRef.current?.(filterDTO);
  }, [autoApply, filterDTO]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(e.target as Node)) {
        // Only close if currently open to avoid unnecessary state churn
        setOpen((prev) => (prev ? false : prev));
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

    if (!autoApply) {
      onChange?.(buildLockedInitialDTO(initialDTO, lockedFieldSet));
    }
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
        onClick={() => setOpen((v) => !v)}
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
        <div className="dropdown-content bg-base-100 rounded-box z-50 w-80 p-0 shadow-xl border border-base-200 mt-2 overflow-hidden flex flex-col">
          {/* Header (Sticky) */}
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
                  onChange={(e) => {
                    if (!hasLockedInitialFilters) {
                      setFilterEnabled(e.target.checked);
                    }
                  }}
                />
              </label>
            </div>
          </div>

          {/* Scrollable Content */}
          <div className="p-4 max-h-[60vh] overflow-y-auto custom-scrollbar">
            {/* Sections */}
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

          {/* Footer actions (Sticky) */}
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
