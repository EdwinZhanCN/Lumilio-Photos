import { useSystemSettings, useUpdateSystemSettings } from "../../api/useSystemSettings";
import { useDraftSettings, type DraftSettings } from "../../hooks/useDraftSettings";

export type GeocodingProvider = "disabled" | "nominatim";

export interface GeocodingSettingsDraft {
  provider: GeocodingProvider;
  nominatim_endpoint: string;
  language: string;
  user_agent: string;
}

function toServerDraft(
  data: ReturnType<typeof useSystemSettings>["data"],
): GeocodingSettingsDraft | undefined {
  const geocoding = data?.geocoding;
  if (!geocoding) return undefined;

  return {
    provider: geocoding.provider === "nominatim" ? "nominatim" : "disabled",
    nominatim_endpoint: geocoding.nominatim_endpoint ?? "",
    language: geocoding.language ?? "",
    user_agent: geocoding.user_agent ?? "",
  };
}

export function useGeocodingSettingsDraft(): DraftSettings<GeocodingSettingsDraft> & {
  query: ReturnType<typeof useSystemSettings>;
} {
  const query = useSystemSettings();
  const saveMutation = useUpdateSystemSettings();
  const server = toServerDraft(query.data);

  const draftSettings = useDraftSettings<GeocodingSettingsDraft>({
    server,
    isLoading: query.isLoading,
    isSaving: saveMutation.isPending,
    saveError: saveMutation.error,
    onSave: async (draft) => {
      await saveMutation.mutateAsync({
        body: {
          geocoding: {
            provider: draft.provider,
            nominatim_endpoint: draft.nominatim_endpoint,
            language: draft.language,
            user_agent: draft.user_agent,
          },
        },
      });
    },
  });

  return { ...draftSettings, query };
}
