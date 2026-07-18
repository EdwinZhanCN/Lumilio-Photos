import { memo, useCallback, useMemo, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { MapComponent } from "../../../../map";
import type { AssetLocationBBox } from "../../../../model/filter";
import { centerToBBox, isZeroBBox } from "../filterState";
import { SectionShell } from "./SectionShell";

interface LocationSectionProps {
  filterDisabled: boolean;
  enabled: boolean;
  onEnabledChange: (value: boolean) => void;
  bbox: AssetLocationBBox;
  onBBoxChange: (bbox: AssetLocationBBox) => void;
}

export const LocationSection = memo(function LocationSection({
  filterDisabled,
  enabled,
  onEnabledChange,
  bbox,
  onBBoxChange,
}: LocationSectionProps) {
  const [mapModalOpen, setMapModalOpen] = useState(false);
  const [locationRadiusKm, setLocationRadiusKm] = useState(5);
  const [locationCenterLat, setLocationCenterLat] = useState(0);
  const [locationCenterLon, setLocationCenterLon] = useState(0);
  const { t } = useI18n();

  const setCurrentLocationAsCenter = useCallback(() => {
    if (!navigator.geolocation) return;
    navigator.geolocation.getCurrentPosition((position) => {
      const { latitude, longitude } = position.coords;
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
              onChange={(event) => onBBoxChange({ ...bbox, north: Number(event.target.value) })}
            />
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder={t("assets.filterTool.locationSection.south_placeholder")}
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.south}
              onChange={(event) => onBBoxChange({ ...bbox, south: Number(event.target.value) })}
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
              onChange={(event) => onBBoxChange({ ...bbox, east: Number(event.target.value) })}
            />
            <input
              type="number"
              className="input input-bordered input-xs w-1/2"
              placeholder={t("assets.filterTool.locationSection.west_placeholder")}
              step="0.000001"
              disabled={filterDisabled || !enabled}
              value={bbox.west}
              onChange={(event) => onBBoxChange({ ...bbox, west: Number(event.target.value) })}
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
                  onChange={(event) => setLocationCenterLat(Number(event.target.value))}
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
                  onChange={(event) => setLocationCenterLon(Number(event.target.value))}
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
                  onChange={(event) => setLocationRadiusKm(Number(event.target.value))}
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
                onClick={() => setMapModalOpen(false)}
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
