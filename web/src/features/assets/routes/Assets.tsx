import { useLocation, useSearchParams } from "react-router-dom";
import { AssetsProvider } from "../AssetsProvider";
import { ErrorBoundary } from "react-error-boundary";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import ErrorFallBack from "@/components/ErrorFallBack";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { PinAssetsHero } from "@/features/assets/components/page/PinAssetsHero";

interface AssetsOrigin {
  from?: string;
  fromLabel?: string;
  label?: string;
}

const Assets = () => {
  const { t } = useI18n();
  const [searchParams] = useSearchParams();
  const location = useLocation();
  const pin = searchParams.get("pin");
  const isPinMode = Boolean(pin);

  // In pin mode the gallery is a leaf reached from elsewhere (typically the
  // agent board). Show a back-crumb to wherever we came from; fall back to the
  // board when the origin state is missing (e.g. on hard refresh / deep-link).
  const origin = (location.state ?? null) as AssetsOrigin | null;
  useBreadcrumbs(
    isPinMode
      ? [
          {
            label: origin?.fromLabel ?? t("lumilio.nav.board", "Board"),
            to: origin?.from ?? "/lumilio",
          },
          { label: origin?.label ?? t("assets.pinTrailLabel", "Selection") },
        ]
      : [],
  );

  // Pin deep-links use an isolated, non-persistent scope so they never pollute
  // the main gallery's persisted filters/sort. syncUrl keeps the `pin` param
  // alive across carousel open/close navigation.
  const scopeId = isPinMode ? `assets:pin:${pin}` : "assets:main";

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <AssetsProvider
        key={scopeId}
        scopeId={scopeId}
        persist={!isPinMode}
        syncUrl={isPinMode}
      >
        <WorkerProvider>
          <AssetsGalleryPage
            pinId={pin ?? undefined}
            hero={isPinMode ? <PinAssetsHero pinId={pin as string} /> : undefined}
          />
        </WorkerProvider>
      </AssetsProvider>
    </ErrorBoundary>
  );
};

export default Assets;
