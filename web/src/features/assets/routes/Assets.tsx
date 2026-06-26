import { useSearchParams } from "react-router-dom";
import { AssetsProvider } from "../AssetsProvider";
import { ErrorBoundary } from "react-error-boundary";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import ErrorFallBack from "@/components/ErrorFallBack";
import { useI18n } from "@/lib/i18n";
import { AssetsGalleryPage } from "@/features/assets/components/page/AssetsGalleryPage";
import { PinAssetsHero } from "@/features/assets/components/page/PinAssetsHero";

const Assets = () => {
  const { t } = useI18n();
  const [searchParams] = useSearchParams();
  const pin = searchParams.get("pin");
  const isPinMode = Boolean(pin);

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
