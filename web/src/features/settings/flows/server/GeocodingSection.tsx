import { AtSignIcon, Globe2Icon, LanguagesIcon, MapPinIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../../components/SettingsGroup";
import { SettingsDropdown } from "../../components/SettingsDropdown";
import { SettingsSaveBar } from "../../components/SettingsSaveBar";
import { useGeocodingSettingsDraft, type GeocodingProvider } from "./useGeocodingSettingsDraft";

export default function GeocodingSection() {
  const { t } = useI18n();
  const {
    draft,
    setField,
    isDirty,
    isLoading,
    isSaving,
    save,
    reset,
    saveError,
    justSaved,
    canSave,
    query,
  } = useGeocodingSettingsDraft();

  if (query.isError) {
    return (
      <SettingsGroup
        title={t("settings.serverSettings.geocoding.title", "Reverse geocoding")}
        description={t(
          "settings.serverSettings.geocoding.description",
          "Enabling a provider sends photo coordinates to the configured endpoint; Disabled keeps them local.",
        )}
      >
        <SettingsBlock>
          <p className="text-sm text-warning">
            {t(
              "settings.serverSettings.geocoding.loadError",
              "Reverse-geocoding settings are temporarily unavailable.",
            )}
          </p>
        </SettingsBlock>
      </SettingsGroup>
    );
  }

  if (isLoading || !draft) {
    return (
      <SettingsGroup
        title={t("settings.serverSettings.geocoding.title", "Reverse geocoding")}
        description={t(
          "settings.serverSettings.geocoding.description",
          "Enabling a provider sends photo coordinates to the configured endpoint; Disabled keeps them local.",
        )}
      >
        <SettingsBlock>
          <p className="text-sm text-base-content/60">{t("common.loading", "Loading...")}</p>
        </SettingsBlock>
      </SettingsGroup>
    );
  }

  const providerOptions: ReadonlyArray<{ value: GeocodingProvider; label: string }> = [
    {
      value: "disabled",
      label: t("settings.serverSettings.geocoding.providers.disabled", "Disabled"),
    },
    {
      value: "nominatim",
      label: t("settings.serverSettings.geocoding.providers.nominatim", "Nominatim"),
    },
  ];

  return (
    <>
      <SettingsGroup
        title={t("settings.serverSettings.geocoding.title", "Reverse geocoding")}
        description={t(
          "settings.serverSettings.geocoding.description",
          "Enabling a provider sends photo coordinates to the configured endpoint; Disabled keeps them local.",
        )}
      >
        <SettingsRow
          icon={<MapPinIcon className="size-4" />}
          iconColor="bg-primary text-primary-content"
          label={t("settings.serverSettings.geocoding.providerLabel", "Provider")}
          description={t(
            "settings.serverSettings.geocoding.providerDescription",
            "Disabled keeps coordinates local and performs no provider requests.",
          )}
          control={
            <SettingsDropdown
              value={draft.provider}
              options={providerOptions}
              onChange={(provider) => setField("provider", provider)}
              disabled={isSaving}
              ariaLabel={t("settings.serverSettings.geocoding.providerLabel", "Provider")}
              className="w-36"
            />
          }
        />

        <SettingsBlock>
          <label
            htmlFor="geocoding-endpoint"
            className="flex items-center gap-2 text-sm font-medium"
          >
            <Globe2Icon className="size-3.5 text-base-content/50" />
            {t("settings.serverSettings.geocoding.endpointLabel", "Nominatim endpoint")}
          </label>
          <input
            id="geocoding-endpoint"
            type="url"
            className="input input-bordered input-sm mt-2 w-full"
            value={draft.nominatim_endpoint}
            maxLength={2048}
            autoComplete="off"
            spellCheck={false}
            disabled={isSaving}
            onChange={(event) => setField("nominatim_endpoint", event.target.value)}
          />
          <p className="mt-1.5 text-xs text-base-content/55">
            {t(
              "settings.serverSettings.geocoding.endpointDescription",
              "Use an absolute HTTP(S) URL. Loopback and private-network endpoints are allowed.",
            )}
          </p>
        </SettingsBlock>

        <SettingsRow
          icon={<LanguagesIcon className="size-4" />}
          iconColor="bg-info text-info-content"
          label={t("settings.serverSettings.geocoding.languageLabel", "Language")}
          description={t(
            "settings.serverSettings.geocoding.languageDescription",
            "Sent as the provider's language preference.",
          )}
          control={
            <input
              type="text"
              className="input input-bordered input-sm w-28"
              value={draft.language}
              maxLength={64}
              list="geocoding-language-options"
              autoComplete="off"
              spellCheck={false}
              disabled={isSaving}
              aria-label={t("settings.serverSettings.geocoding.languageLabel", "Language")}
              onChange={(event) => setField("language", event.target.value)}
            />
          }
        />
        <datalist id="geocoding-language-options">
          <option value="en" />
          <option value="zh" />
        </datalist>

        <SettingsBlock>
          <label
            htmlFor="geocoding-user-agent"
            className="flex items-center gap-2 text-sm font-medium"
          >
            <AtSignIcon className="size-3.5 text-base-content/50" />
            {t("settings.serverSettings.geocoding.userAgentLabel", "User-Agent")}
          </label>
          <input
            id="geocoding-user-agent"
            type="text"
            className="input input-bordered input-sm mt-2 w-full"
            value={draft.user_agent}
            maxLength={512}
            autoComplete="off"
            spellCheck={false}
            disabled={isSaving}
            onChange={(event) => setField("user_agent", event.target.value)}
          />
          <p className="mt-1.5 text-xs text-base-content/55">
            {t(
              "settings.serverSettings.geocoding.userAgentDescription",
              "Requests identify themselves with this value. Do not include credentials or personal data.",
            )}
          </p>
        </SettingsBlock>
      </SettingsGroup>

      <SettingsSaveBar
        isDirty={isDirty}
        isSaving={isSaving}
        justSaved={justSaved}
        error={saveError}
        canSave={canSave}
        onSave={save}
        onReset={reset}
      />
    </>
  );
}
