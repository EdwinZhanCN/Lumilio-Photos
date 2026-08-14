export type CurrentLocationCapability = "available" | "secure-context-required" | "unsupported";

type CurrentLocationEnvironment = {
  secureContext: boolean;
  geolocationAvailable: boolean;
};

export function getCurrentLocationCapability(
  environment: CurrentLocationEnvironment = {
    secureContext: typeof window !== "undefined" && window.isSecureContext,
    geolocationAvailable: typeof navigator !== "undefined" && Boolean(navigator.geolocation),
  },
): CurrentLocationCapability {
  if (!environment.secureContext) return "secure-context-required";
  if (!environment.geolocationAvailable) return "unsupported";
  return "available";
}
