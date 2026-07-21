import type { TFunction } from "i18next";

export type ProviderTextResolver = (
  key?: string,
  params?: Record<string, string>,
) => string;

/**
 * Resolves backend-emitted i18n keys for cloud provider descriptors
 * (titles, field labels, challenge text). Known keys are registered as
 * literal t() calls so the i18next extractor picks them up; unknown keys
 * (future providers) degrade to the raw key string.
 */
export function createProviderTextResolver(t: TFunction): ProviderTextResolver {
  const defaults: Record<string, string> = {
    "cloudProvider.icloud.title": t("cloudProvider.icloud.title", "iCloud"),
    "cloudProvider.icloud.description": t(
      "cloudProvider.icloud.description",
      "Import originals from iCloud Photos.",
    ),
    "cloudProvider.icloud.securityNote": t(
      "cloudProvider.icloud.securityNote",
      "Lumilio uses the password only during authentication and stores the resulting session in an isolated credential directory.",
    ),
    "cloudProvider.icloud.field.username": t(
      "cloudProvider.icloud.field.username",
      "Apple ID",
    ),
    "cloudProvider.icloud.field.password": t(
      "cloudProvider.icloud.field.password",
      "Password",
    ),
    "cloudProvider.icloud.field.domain": t(
      "cloudProvider.icloud.field.domain",
      "Apple domain",
    ),
    "cloudProvider.icloud.option.domain.com": t(
      "cloudProvider.icloud.option.domain.com",
      "Global iCloud",
    ),
    "cloudProvider.icloud.option.domain.cn": t(
      "cloudProvider.icloud.option.domain.cn",
      "Mainland China iCloud",
    ),
    "cloudProvider.icloud.challenge.field.code": t(
      "cloudProvider.icloud.challenge.field.code",
      "Verification code",
    ),
    "cloudProvider.icloud.challenge.sms.title": t(
      "cloudProvider.icloud.challenge.sms.title",
      "Verification required",
    ),
    "cloudProvider.icloud.challenge.sms.description": t(
      "cloudProvider.icloud.challenge.sms.description",
      "Enter the verification code sent to {{phone}}.",
    ),
  };
  return (key, params) => {
    if (!key) return "";
    return t(key, { ...params, defaultValue: defaults[key] ?? key });
  };
}
