import type { CloudProviderField } from "@/features/cloud";

export type CloudCredentialFormValues = Record<string, string>;

export function createCloudCredentialFormValues(
  fields: CloudProviderField[] = [],
): CloudCredentialFormValues {
  return fields.reduce<CloudCredentialFormValues>((values, field) => {
    if (field.name) {
      values[field.name] = field.type === "select" ? (field.options?.[0]?.value ?? "") : "";
    }
    return values;
  }, {});
}

export function areRequiredCloudFieldsFilled(
  fields: CloudProviderField[] = [],
  values: CloudCredentialFormValues,
) {
  return fields.every((field) => !field.required || !field.name || values[field.name]?.trim());
}
