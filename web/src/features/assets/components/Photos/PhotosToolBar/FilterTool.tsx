import { ListFilterIcon } from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { assetService } from "@/services/assetsService";

/* =========================
   Types
   ========================= */

type FilenameOperator = "contains" | "matches" | "starts_with" | "ends_with";

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
  raw?: boolean;
  rating?: number; // 0-5, where 0 means unrated
  liked?: boolean;
  filename?: FilenameFilter;
  date?: DateRange;
  camera_make?: string;
  lens?: string;

  // Extended field to represent spatial filtering
  location?: LocationBBox;
}

type FilterToolProps = {
  initial?: FilterDTO;
  onChange?: (filters: FilterDTO) => void;
  autoApply?: boolean;

  // Options can be provided directly via props, or fetched via provided functions, or fetched from default endpoints.
  cameraMakeOptions?: string[];
  lensOptions?: string[];
  fetchCameraMakes?: () => Promise<string[]>;
  fetchLenses?: () => Promise<string[]>;
};

/* =========================
   Pure helpers
   ========================= */

function centerToBBox(
  lat: number,
  lon: number,
  radiusKm: number,
): LocationBBox {
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

/* =========================
   Small hook: options loading
   ========================= */

function useFilterOptions({
  open,
  cameraMakeOptions,
  lensOptions,
  fetchCameraMakes,
  fetchLenses,
}: {
  open: boolean;
  cameraMakeOptions?: string[];
  lensOptions?: string[];
  fetchCameraMakes?: () => Promise<string[]>;
  fetchLenses?: () => Promise<string[]>;
}) {
  const [cameraMakeItems, setCameraMakeItems] = useState<string[]>(
    cameraMakeOptions ?? [],
  );
  const [lensItems, setLensItems] = useState<string[]>(lensOptions ?? []);
  const [loadingOptions, setLoadingOptions] = useState<boolean>(false);
  const optionsLoadedRef = useRef<boolean>(false);

  useEffect(() => {
    const shouldFetch =
      open &&
      !optionsLoadedRef.current &&
      (!cameraMakeOptions ||
        cameraMakeOptions.length === 0 ||
        !lensOptions ||
        lensOptions.length === 0);
    if (!shouldFetch) return;

    let running = true;
    const load = async () => {
      try {
        setLoadingOptions(true);

        let cm: string[] = cameraMakeOptions ?? [];
        let ln: string[] = lensOptions ?? [];

        if (cm.length === 0 || ln.length === 0) {
          if (fetchCameraMakes && fetchLenses) {
            cm = await fetchCameraMakes();
            ln = await fetchLenses();
          } else {
            // Use the new filter options API
            const response = await assetService.getFilterOptions();
            if (response.data.code === 0 && response.data.data) {
              if (cm.length === 0) {
                cm = response.data.data.camera_makes || [];
              }
              if (ln.length === 0) {
                ln = response.data.data.lenses || [];
              }
            }
          }
        }

        if (running) {
          setCameraMakeItems(cm);
          setLensItems(ln);
          optionsLoadedRef.current = true;
        }
      } catch {
        // ignore
      } finally {
        if (running) setLoadingOptions(false);
      }
    };
    void load();

    return () => {
      running = false;
    };
  }, [open, cameraMakeOptions, lensOptions, fetchCameraMakes, fetchLenses]);

  return { cameraMakeItems, lensItems, loadingOptions };
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
  return (
    <div className="form-control mb-3">
      <div className="flex items-center justify-between">
        <span className="label-text font-medium">{title}</span>
        <label className="label cursor-pointer p-0 gap-2">
          <span className="label-text">Enable</span>
          <input
            type="checkbox"
            className="toggle toggle-primary"
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
  return (
    <SectionShell
      title="RAW"
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join">
        <button
          type="button"
          className={`btn btn-xs join-item ${mode === "include" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onModeChange("include")}
        >
          Include
        </button>
        <button
          type="button"
          className={`btn btn-xs join-item ${mode === "exclude" ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onModeChange("exclude")}
        >
          Exclude
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
  return (
    <SectionShell
      title="Rating"
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join">
        {[5, 4, 3, 2, 1].map((n) => (
          <button
            key={n}
            type="button"
            className={`btn btn-xs join-item ${value === n ? "btn-primary" : "btn-outline"}`}
            disabled={filterDisabled || !enabled}
            onClick={() => onValueChange(n)}
            title={`Rating ${n}`}
          >
            {n}
          </button>
        ))}
        <button
          type="button"
          className={`btn btn-xs join-item ${value === 0 ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(0)}
          title="Unrated"
        >
          U
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
  return (
    <SectionShell
      title="Like"
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="join">
        <button
          type="button"
          className={`btn btn-xs join-item ${value ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(true)}
        >
          Liked
        </button>
        <button
          type="button"
          className={`btn btn-xs join-item ${!value ? "btn-primary" : "btn-outline"}`}
          disabled={filterDisabled || !enabled}
          onClick={() => onValueChange(false)}
        >
          Unliked
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
  return (
    <SectionShell
      title="Filename"
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="flex gap-2">
        <select
          className="select select-bordered select-xs w-1/3"
          disabled={filterDisabled || !enabled}
          value={operator}
          onChange={(e) => onOperatorChange(e.target.value as FilenameOperator)}
        >
          <option value="contains">Contains</option>
          <option value="matches">Matches</option>
          <option value="starts_with">Starts with</option>
          <option value="ends_with">Ends with</option>
        </select>
        <input
          type="text"
          className="input input-bordered input-xs w-2/3"
          placeholder="Enter text..."
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
  return (
    <SectionShell
      title="Date"
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <div className="flex gap-2">
        <label className="input input-bordered input-xs flex-1 flex items-center gap-2">
          <span className="text-xs opacity-70">From</span>
          <input
            type="date"
            className="grow text-xs"
            value={from}
            disabled={filterDisabled || !enabled}
            onChange={(e) => onFromChange(e.target.value)}
          />
        </label>
        <label className="input input-bordered input-xs flex-1 flex items-center gap-2">
          <span className="text-xs opacity-70">To</span>
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

  const setCurrentLocationAsCenter = useCallback(() => {
    if (!navigator.geolocation) return;
    navigator.geolocation.getCurrentPosition((pos) => {
      const { latitude, longitude } = pos.coords;
      setLocationCenterLat(latitude);
      setLocationCenterLon(longitude);
    });
  }, []);

  const computeBBoxFromCenter = useCallback(() => {
    const newBBox = centerToBBox(
      locationCenterLat,
      locationCenterLon,
      locationRadiusKm,
    );
    onBBoxChange(newBBox);
  }, [locationCenterLat, locationCenterLon, locationRadiusKm, onBBoxChange]);

  return (
    <>
      <SectionShell
        title="Location"
        enabled={enabled}
        onToggle={onEnabledChange}
        disabled={filterDisabled}
      >
        <div className="flex flex-col gap-2">
          <div className="flex gap-2">
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder="North"
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.north}
              onChange={(e) =>
                onBBoxChange({ ...bbox, north: Number(e.target.value) })
              }
            />
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder="South"
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.south}
              onChange={(e) =>
                onBBoxChange({ ...bbox, south: Number(e.target.value) })
              }
            />
          </div>
          <div className="flex gap-2">
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder="East"
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.east}
              onChange={(e) =>
                onBBoxChange({ ...bbox, east: Number(e.target.value) })
              }
            />
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder="West"
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.west}
              onChange={(e) =>
                onBBoxChange({ ...bbox, west: Number(e.target.value) })
              }
            />
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              className="btn btn-xs btn-outline"
              disabled={filterDisabled || !enabled}
              onClick={() => setMapModalOpen(true)}
            >
              Pick on map
            </button>
            <button
              type="button"
              className="btn btn-xs btn-ghost"
              disabled={filterDisabled || !enabled}
              onClick={setCurrentLocationAsCenter}
            >
              Use current location
            </button>
          </div>
        </div>
      </SectionShell>

      {mapModalOpen && (
        <dialog className="modal modal-open">
          <div className="modal-box">
            <h3 className="font-bold text-lg">Pick location</h3>
            <p className="py-2 text-sm opacity-80">
              Provide a center + radius to generate a bounding box, or manually
              edit the values.
            </p>

            <div className="grid grid-cols-2 gap-2 mt-2">
              <label className="form-control">
                <span className="label-text">Center lat</span>
                <input
                  type="number"
                  className="input input-bordered input-sm"
                  step="0.000001"
                  value={locationCenterLat}
                  onChange={(e) => setLocationCenterLat(Number(e.target.value))}
                />
              </label>

              <label className="form-control">
                <span className="label-text">Center lon</span>
                <input
                  type="number"
                  className="input input-bordered input-sm"
                  step="0.000001"
                  value={locationCenterLon}
                  onChange={(e) => setLocationCenterLon(Number(e.target.value))}
                />
              </label>

              <label className="form-control col-span-2">
                <span className="label-text">Radius (km)</span>
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
                className="btn btn-sm btn-outline"
                onClick={setCurrentLocationAsCenter}
              >
                Use current location
              </button>
              <button
                type="button"
                className="btn btn-sm"
                onClick={computeBBoxFromCenter}
              >
                Generate BBox
              </button>
            </div>

            <div className="mt-4">
              <div className="text-sm opacity-70 mb-2">Preview map</div>
              <div className="w-full h-48 rounded-box overflow-hidden border border-base-300">
                <iframe
                  title="map"
                  className="w-full h-full"
                  src={`https://www.openstreetmap.org/export/embed.html?bbox=${encodeURIComponent(
                    (bbox.west || -180).toString(),
                  )}%2C${encodeURIComponent(
                    (bbox.south || -85).toString(),
                  )}%2C${encodeURIComponent(
                    (bbox.east || 180).toString(),
                  )}%2C${encodeURIComponent(
                    (bbox.north || 85).toString(),
                  )}&layer=mapnik`}
                />
              </div>
              <div className="text-xs opacity-70 mt-2">
                BBox N:{bbox.north.toFixed(6)} S:{bbox.south.toFixed(6)} E:
                {bbox.east.toFixed(6)} W:
                {bbox.west.toFixed(6)}
              </div>
            </div>

            <div className="modal-action">
              <button
                type="button"
                className="btn btn-ghost"
                onClick={() => setMapModalOpen(false)}
              >
                Cancel
              </button>
              <button
                type="button"
                className="btn btn-primary"
                onClick={() => {
                  setMapModalOpen(false);
                }}
              >
                Done
              </button>
            </div>
          </div>
          <form
            method="dialog"
            className="modal-backdrop"
            onClick={() => setMapModalOpen(false)}
          >
            <button>close</button>
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
  return (
    <SectionShell
      title="Camera Make"
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <select
        className="select select-bordered select-xs"
        disabled={filterDisabled || !enabled || loading}
        value={value}
        onChange={(e) => onValueChange(e.target.value)}
      >
        <option value="">Select camera make</option>
        {items.map((item) => (
          <option key={item} value={item}>
            {item}
          </option>
        ))}
      </select>
      {loading && (
        <span className="text-xs opacity-70 mt-1 block">Loading options…</span>
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
  return (
    <SectionShell
      title="Lens"
      enabled={enabled}
      onToggle={onEnabledChange}
      disabled={filterDisabled}
    >
      <select
        className="select select-bordered select-xs"
        disabled={filterDisabled || !enabled || loading}
        value={value}
        onChange={(e) => onValueChange(e.target.value)}
      >
        <option value="">Select lens</option>
        {items.map((item) => (
          <option key={item} value={item}>
            {item}
          </option>
        ))}
      </select>
      {loading && (
        <span className="text-xs opacity-70 mt-1 block">Loading options…</span>
      )}
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
  cameraMakeOptions,
  lensOptions,
  fetchCameraMakes,
  fetchLenses,
}: FilterToolProps) {
  // Dropdown open state (independent of filter enabled)
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);

  // Global filter enable/disable
  const [filterEnabled, setFilterEnabled] = useState<boolean>(
    !!initial && Object.keys(initial).length > 0,
  );

  // RAW
  const [rawEnabled, setRawEnabled] = useState<boolean>(
    typeof initial?.raw === "boolean",
  );
  const [rawMode, setRawMode] = useState<"include" | "exclude">(
    initial?.raw === false ? "exclude" : "include",
  );

  // Rating: 5/4/3/2/1/unrated(0)
  const [ratingEnabled, setRatingEnabled] = useState<boolean>(
    typeof initial?.rating === "number",
  );
  const [ratingValue, setRatingValue] = useState<number>(
    typeof initial?.rating === "number" ? initial!.rating! : 5,
  );

  // Liked: liked/unliked
  const [likedEnabled, setLikedEnabled] = useState<boolean>(
    typeof initial?.liked === "boolean",
  );
  const [likedValue, setLikedValue] = useState<boolean>(initial?.liked ?? true);

  // Filename: operator + value
  const [filenameEnabled, setFilenameEnabled] = useState<boolean>(
    !!initial?.filename,
  );
  const [filenameOperator, setFilenameOperator] = useState<FilenameOperator>(
    initial?.filename?.operator ?? "contains",
  );
  const [filenameValue, setFilenameValue] = useState<string>(
    initial?.filename?.value ?? "",
  );

  // Date range
  const [dateEnabled, setDateEnabled] = useState<boolean>(!!initial?.date);
  const [dateFrom, setDateFrom] = useState<string>(initial?.date?.from ?? "");
  const [dateTo, setDateTo] = useState<string>(initial?.date?.to ?? "");

  // Location (BBox)
  const [locationEnabled, setLocationEnabled] = useState<boolean>(
    !!initial?.location,
  );
  const [location, setLocation] = useState<LocationBBox>(
    initial?.location ?? { north: 0, south: 0, east: 0, west: 0 },
  );

  // Camera make / Lens
  const [cameraMakeEnabled, setCameraMakeEnabled] = useState<boolean>(
    !!initial?.camera_make,
  );
  const [cameraMake, setCameraMake] = useState<string>(
    initial?.camera_make ?? "",
  );
  const [lensEnabled, setLensEnabled] = useState<boolean>(!!initial?.lens);
  const [lens, setLens] = useState<string>(initial?.lens ?? "");

  // Options hook
  const { cameraMakeItems, lensItems, loadingOptions } = useFilterOptions({
    open,
    cameraMakeOptions,
    lensOptions,
    fetchCameraMakes,
    fetchLenses,
  });

  const enabledCount = useMemo(() => {
    if (!filterEnabled) return 0;
    let count = 0;
    if (rawEnabled) count++;
    if (ratingEnabled) count++;
    if (likedEnabled) count++;
    if (filenameEnabled && filenameValue.trim() !== "") count++;
    if (dateEnabled && (dateFrom || dateTo)) count++;
    if (locationEnabled && !isZeroBBox(location)) count++;
    if (cameraMakeEnabled && cameraMake) count++;
    if (lensEnabled && lens) count++;
    return count;
  }, [
    filterEnabled,
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
    cameraMakeEnabled,
    cameraMake,
    lensEnabled,
    lens,
  ]);

  const buildDTO = useCallback((): FilterDTO => {
    if (!filterEnabled) return {};
    const dto: FilterDTO = {};
    if (rawEnabled) dto.raw = rawMode === "include";
    if (ratingEnabled) dto.rating = ratingValue;
    if (likedEnabled) dto.liked = likedValue;
    if (filenameEnabled && filenameValue.trim()) {
      dto.filename = {
        operator: filenameOperator,
        value: filenameValue.trim(),
      };
    }
    if (dateEnabled && (dateFrom || dateTo)) {
      dto.date = {
        from: dateFrom || undefined,
        to: dateTo || undefined,
      };
    }
    if (locationEnabled) {
      dto.location = { ...location };
    }
    if (cameraMakeEnabled && cameraMake) dto.camera_make = cameraMake;
    if (lensEnabled && lens) dto.lens = lens;
    return dto;
  }, [
    filterEnabled,
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
    cameraMakeEnabled,
    cameraMake,
    lensEnabled,
    lens,
  ]);

  const emit = useCallback(() => {
    onChange?.(buildDTO());
  }, [onChange, buildDTO]);

  // Auto-emit on state change if enabled
  useEffect(() => {
    if (autoApply) {
      emit();
    }
  }, [autoApply, emit]);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (!rootRef.current) return;
      if (!rootRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  const resetAll = useCallback(() => {
    setFilterEnabled(false);

    setRawEnabled(false);
    setRawMode("include");

    setRatingEnabled(false);
    setRatingValue(5);

    setLikedEnabled(false);
    setLikedValue(true);

    setFilenameEnabled(false);
    setFilenameOperator("contains");
    setFilenameValue("");

    setDateEnabled(false);
    setDateFrom("");
    setDateTo("");

    setLocationEnabled(false);
    setLocation({ north: 0, south: 0, east: 0, west: 0 });

    setCameraMakeEnabled(false);
    setCameraMake("");

    setLensEnabled(false);
    setLens("");

    if (!autoApply) {
      onChange?.({});
    }
  }, [autoApply, onChange]);

  const applyNow = useCallback(() => {
    emit();
    setOpen(false);
  }, [emit]);

  return (
    <div
      className={`dropdown dropdown-start ${open ? "dropdown-open" : ""}`}
      ref={rootRef}
    >
      <button
        type="button"
        className={`btn btn-sm btn-circle btn-soft btn-info ${filterEnabled ? "btn-active" : ""} relative`}
        aria-pressed={filterEnabled}
        onClick={() => setOpen((v) => !v)}
        title="Filters"
      >
        <ListFilterIcon />
        {enabledCount > 0 && (
          <span className="badge badge-xs badge-primary absolute -right-1 -top-1">
            {enabledCount}
          </span>
        )}
      </button>

      <div className="dropdown-content bg-base-100 rounded-box z-50 w-96 p-3 shadow">
        {/* Header */}
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <span className="font-medium">Filters</span>
            <span className="badge badge-ghost">{enabledCount} active</span>
          </div>
          <div className="flex items-center gap-2">
            <label className="label cursor-pointer p-0 gap-2">
              <span className="label-text">Enable</span>
              <input
                type="checkbox"
                className="toggle toggle-primary"
                checked={filterEnabled}
                onChange={(e) => setFilterEnabled(e.target.checked)}
              />
            </label>
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={() => setOpen(false)}
            >
              Close
            </button>
          </div>
        </div>

        <div className="divider my-2" />

        {/* Sections */}
        <RawSection
          filterDisabled={!filterEnabled}
          enabled={rawEnabled}
          onEnabledChange={setRawEnabled}
          mode={rawMode}
          onModeChange={setRawMode}
        />

        <RatingSection
          filterDisabled={!filterEnabled}
          enabled={ratingEnabled}
          onEnabledChange={setRatingEnabled}
          value={ratingValue}
          onValueChange={setRatingValue}
        />

        <LikeSection
          filterDisabled={!filterEnabled}
          enabled={likedEnabled}
          onEnabledChange={setLikedEnabled}
          value={likedValue}
          onValueChange={setLikedValue}
        />

        <FilenameSection
          filterDisabled={!filterEnabled}
          enabled={filenameEnabled}
          onEnabledChange={setFilenameEnabled}
          operator={filenameOperator}
          onOperatorChange={setFilenameOperator}
          value={filenameValue}
          onValueChange={setFilenameValue}
        />

        <DateSection
          filterDisabled={!filterEnabled}
          enabled={dateEnabled}
          onEnabledChange={setDateEnabled}
          from={dateFrom}
          onFromChange={setDateFrom}
          to={dateTo}
          onToChange={setDateTo}
        />

        <LocationSection
          filterDisabled={!filterEnabled}
          enabled={locationEnabled}
          onEnabledChange={setLocationEnabled}
          bbox={location}
          onBBoxChange={setLocation}
        />

        <CameraMakeSection
          filterDisabled={!filterEnabled}
          enabled={cameraMakeEnabled}
          onEnabledChange={setCameraMakeEnabled}
          value={cameraMake}
          onValueChange={setCameraMake}
          items={cameraMakeItems}
          loading={loadingOptions}
        />

        <LensSection
          filterDisabled={!filterEnabled}
          enabled={lensEnabled}
          onEnabledChange={setLensEnabled}
          value={lens}
          onValueChange={setLens}
          items={lensItems}
          loading={loadingOptions}
        />

        <div className="divider my-2" />

        {/* Footer actions */}
        <div className="flex items-center justify-between">
          <button
            type="button"
            className="btn btn-xs btn-outline"
            onClick={resetAll}
          >
            Reset
          </button>
          {!autoApply && (
            <button
              type="button"
              className="btn btn-xs btn-primary"
              onClick={applyNow}
            >
              Apply
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
