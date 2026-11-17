import { useEffect, useState, type CSSProperties } from "react";
import "./tray.css";
import { useRive } from "@rive-app/react-canvas";
import { trayLayers, trayProgress, trayTier } from "../../model/processingProgress";
import { PROCESSING_ARTBOARD, PROCESSING_STATE_MACHINE, processingAssetUrl } from "./runtime";

/**
 * The Processing files tray.
 *
 * Decorative backlog indicator for files awaiting processing, the one kind of
 * work the Processing tab animates. The owning card renders the name, exact
 * count, and any attention state independently.
 */

export interface ProcessingTrayProps {
  pending: number;
  /** Overridable so a spec can exercise the static fallback. */
  assetUrl?: string;
}

/** True when the reader asked for reduced motion; the static tray is equivalent. */
function usePrefersReducedMotion(): boolean {
  const [reduced, setReduced] = useState(
    () =>
      typeof window !== "undefined" &&
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-reduced-motion: reduce)").matches,
  );

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    const query = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => setReduced(query.matches);
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);

  return reduced;
}

export function ProcessingTray({ pending, assetUrl }: ProcessingTrayProps) {
  const resolvedAssetUrl = assetUrl ?? processingAssetUrl();
  const reducedMotion = usePrefersReducedMotion();
  const [failed, setFailed] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const [buffer, setBuffer] = useState<ArrayBuffer>();
  const wantsAnimation = !reducedMotion && !!resolvedAssetUrl;
  useEffect(() => {
    if (!wantsAnimation || !resolvedAssetUrl) return;
    const controller = new AbortController();
    setBuffer(undefined);
    setFailed(false);
    setLoaded(false);
    void fetch(resolvedAssetUrl, { signal: controller.signal })
      .then((response) => {
        if (!response.ok) throw new Error("Tray asset unavailable");
        return response.arrayBuffer();
      })
      .then((value) => {
        if (!controller.signal.aborted) setBuffer(value);
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [wantsAnimation, resolvedAssetUrl]);

  const progress = trayProgress(pending);
  const layers = trayLayers(pending);
  const tier = trayTier(progress);
  const animate = wantsAnimation && !failed;

  const { rive, RiveComponent } = useRive(
    animate && buffer
      ? {
          buffer,
          artboard: PROCESSING_ARTBOARD,
          stateMachine: PROCESSING_STATE_MACHINE,
          autoplay: false,
          autoBind: true,
          shouldDisableRiveListeners: true,
          onRiveReady: (instance) => {
            const value = instance.viewModelInstance?.number("progress");
            if (!value) {
              setFailed(true);
              return;
            }
            value.value = progress;
            instance.play(PROCESSING_STATE_MACHINE);
            setLoaded(true);
          },
          onLoadError: () => setFailed(true),
        }
      : null,
  );

  useEffect(() => {
    const value = rive?.viewModelInstance?.number("progress");
    if (value) value.value = progress;
  }, [rive, progress]);

  return (
    <figure
      aria-hidden="true"
      data-tier={tier}
      data-layers={layers}
      data-animated={animate}
      data-loaded={loaded}
      className="m-0 aspect-square w-full self-center"
    >
      <div aria-hidden="true" className="aspect-square w-full">
        {animate && buffer ? (
          <RiveComponent style={{ width: "100%", height: "100%" }} />
        ) : (
          <StaticTray layers={layers} />
        )}
      </div>
    </figure>
  );
}

/** Static fallback when motion is reduced or the runtime cannot load. */
function StaticTray({ layers }: { layers: number }) {
  return (
    <div className="monitor-static-tray">
      <div className="monitor-tray-base" />
      {Array.from({ length: 18 }, (_, index) => (
        <span
          key={index}
          className="monitor-tray-sheet"
          data-visible={index < layers}
          style={{ "--layer": index } as CSSProperties}
        />
      ))}
      <div className="monitor-tray-front" />
    </div>
  );
}
