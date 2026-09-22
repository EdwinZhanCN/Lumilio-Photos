import { FolderOpen } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { type DesktopSnapshot } from "../../bindings/desktop/internal/control/dto/models.js";
import { AnimatedBadge } from "@/components/motion/animated-badge";
import { Button } from "@/components/motion/button";
import { Loader } from "@/components/motion/loader";
import {
  InlineNotice,
  PageHeading,
  SettingRow,
  SettingsSection,
} from "@/components/settings/setting-layout";
import { type SettingsDraftController } from "@/features/settings/use-settings-draft";
import { appIconURL } from "./shared";
import { LanguageSelect, NetworkSelect, RegionSelect, networkLabel } from "./settingsControls";

export function Onboarding({
  snapshot,
  draft,
  onComplete,
}: {
  snapshot: DesktopSnapshot;
  draft: SettingsDraftController;
  onComplete: () => void;
}) {
  const { t } = useTranslation();
  const [step, setStep] = useState(0);
  const total = 4;

  if (!draft.runtime || !draft.preferences) {
    return (
      <div className="bootstrap-loading">
        <img
          src={appIconURL}
          alt={t("product.compactName", "Lumilio Photos")}
          className="bootstrap-icon"
        />
        <Loader variant="ascii-braille" size={24} label={t("onboarding.preparing")} />
        <span>{t("onboarding.preparingDescription")}</span>
      </div>
    );
  }

  const finish = async () => {
    if (await draft.save()) onComplete();
  };
  const busy = draft.phase === "preparing" || draft.phase === "saving";

  return (
    <section
      className="bootstrap"
      aria-label={t("onboarding.ariaLabel", "Lumilio Photos Desktop setup")}
    >
      <header className="bootstrap-header">
        <div className="bootstrap-brand">
          <img src={appIconURL} alt="" className="bootstrap-icon" />
          <span>{t("product.compactName", "Lumilio Photos")}</span>
        </div>
        <span>{t("onboarding.step", { current: step + 1, total })}</span>
      </header>

      <div className="bootstrap-progress" aria-hidden>
        {Array.from({ length: total }, (_, index) => (
          <i key={index} className={index <= step ? "active" : ""} />
        ))}
      </div>

      <AnimatePresence mode="wait" initial={false}>
        <motion.div
          key={step}
          className="bootstrap-slide"
          initial={{ opacity: 0, x: 18 }}
          animate={{ opacity: 1, x: 0 }}
          exit={{ opacity: 0, x: -14 }}
          transition={{ duration: 0.2 }}
        >
          {step === 0 ? (
            <>
              <PageHeading
                title={t("onboarding.generalTitle")}
                description={t("onboarding.generalDescription")}
              />
              <SettingsSection title={t("onboarding.desktop")}>
                <SettingRow
                  title={t("onboarding.language")}
                  description={t("onboarding.languageDescription")}
                >
                  <LanguageSelect
                    preferences={draft.preferences}
                    update={draft.updatePreference}
                    disabled={busy}
                  />
                </SettingRow>
                <SettingRow
                  title={t("onboarding.region")}
                  description={t("onboarding.regionDescription")}
                >
                  <RegionSelect
                    preferences={draft.preferences}
                    update={draft.updatePreference}
                    disabled={busy}
                  />
                </SettingRow>
              </SettingsSection>
            </>
          ) : step === 1 ? (
            <>
              <PageHeading
                title={t("onboarding.storageTitle")}
                description={t("onboarding.storageDescription")}
              />
              <SettingsSection title={t("onboarding.storage")}>
                <SettingRow
                  title={t("onboarding.storageLocation")}
                  description={draft.runtime.storagePath}
                >
                  <Button
                    variant="secondary"
                    disabled={busy}
                    onClick={() => void draft.chooseDefaultStorage()}
                  >
                    <FolderOpen className="size-4" /> {t("common.choose")}
                  </Button>
                </SettingRow>
              </SettingsSection>
            </>
          ) : step === 2 ? (
            <>
              <PageHeading
                title={t("onboarding.networkTitle")}
                description={t("onboarding.networkDescription")}
              />
              <SettingsSection title={t("onboarding.server")}>
                <SettingRow
                  title={t("onboarding.networkAccess")}
                  description={t("onboarding.networkRecommended")}
                >
                  <NetworkSelect
                    settings={draft.runtime}
                    update={draft.updateRuntime}
                    disabled={busy}
                  />
                </SettingRow>
              </SettingsSection>
            </>
          ) : (
            <>
              <PageHeading
                title={t("onboarding.readyTitle")}
                description={t("onboarding.readyDescription")}
              />
              <SettingsSection title={t("onboarding.summary")}>
                <SettingRow
                  title={t("onboarding.storageLocation")}
                  description={draft.runtime.storagePath}
                >
                  <span className="setting-value">{t("onboarding.selected")}</span>
                </SettingRow>
                <SettingRow
                  title={t("onboarding.networkAccess")}
                  description={networkLabel(draft.runtime.networkMode, t)}
                >
                  <span className="setting-value">{t("onboarding.configured")}</span>
                </SettingRow>
                <SettingRow
                  title={t("onboarding.lumenAI")}
                  description={
                    snapshot.lumen.installerAvailable && snapshot.lumen.processAvailable
                      ? t("onboarding.lumenAvailable")
                      : t("onboarding.lumenNotIncluded")
                  }
                >
                  <AnimatedBadge status="neutral" size="sm">
                    {snapshot.lumen.installerAvailable && snapshot.lumen.processAvailable
                      ? t("onboarding.optional")
                      : t("onboarding.unavailable")}
                  </AnimatedBadge>
                </SettingRow>
              </SettingsSection>
              {draft.phase === "error" && draft.error ? (
                <InlineNotice tone="danger" title={t("onboarding.setupFailed")}>
                  {draft.error}
                </InlineNotice>
              ) : null}
            </>
          )}
        </motion.div>
      </AnimatePresence>

      <footer className="bootstrap-actions">
        <Button
          variant="ghost"
          disabled={step === 0 || busy}
          onClick={() => setStep((current) => current - 1)}
        >
          {t("common.back")}
        </Button>
        {step < total - 1 ? (
          <Button
            disabled={busy || (step === 1 && !draft.runtime.storagePath)}
            onClick={() => setStep((current) => current + 1)}
          >
            {t("common.continue")}
          </Button>
        ) : (
          <Button
            disabled={busy || draft.validation?.valid === false}
            onClick={() => void finish()}
          >
            {draft.phase === "saving" ? t("common.finishing") : t("onboarding.finishSetup")}
          </Button>
        )}
      </footer>
    </section>
  );
}
