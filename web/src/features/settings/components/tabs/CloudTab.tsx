import { useMemo, useState, type FormEvent } from "react";
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  CloudIcon,
  KeyRoundIcon,
  LogOutIcon,
  PlusIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
  Trash2Icon,
  XIcon,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useCloudCredentials,
  useCloudProviders,
  useCreateCloudCredential,
  useDisconnectCloudCredential,
  useReconnectCloudCredential,
  useRemoveCloudCredential,
  useVerifyCloudCredentialChallenge,
  type CloudAuthChallenge,
  type CloudCredential,
  type CloudProvider,
  type CloudProviderField,
} from "@/features/cloud";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../SettingsGroup";
import { SettingsDropdown } from "../SettingsDropdown";

type FormValues = Record<string, string>;

const credentialStatusClass = (status?: string) => {
  switch (status) {
    case "connected":
      return "badge-success";
    case "pending_challenge":
      return "badge-warning";
    case "error":
      return "badge-error";
    default:
      return "badge-ghost";
  }
};

const fieldInitialValues = (fields: CloudProviderField[] = []): FormValues =>
  fields.reduce<FormValues>((acc, field) => {
    if (field.name) {
      acc[field.name] = field.type === "select" ? (field.options?.[0]?.value ?? "") : "";
    }
    return acc;
  }, {});

const requiredFieldsFilled = (fields: CloudProviderField[] = [], values: FormValues) =>
  fields.every((field) => !field.required || !field.name || values[field.name]?.trim());

export default function CloudTab() {
  const { t } = useI18n();
  const providersQuery = useCloudProviders();
  const credentialsQuery = useCloudCredentials();
  const createCredential = useCreateCloudCredential();
  const verifyChallenge = useVerifyCloudCredentialChallenge();
  const disconnectCredential = useDisconnectCloudCredential();
  const reconnectCredential = useReconnectCloudCredential();
  const removeCredential = useRemoveCloudCredential();

  const [isAddOpen, setIsAddOpen] = useState(false);
  const [providerChoice, setProviderChoice] = useState<string | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [formValues, setFormValues] = useState<FormValues>({});
  const [pendingCredential, setPendingCredential] = useState<CloudCredential | null>(null);
  const [pendingChallenge, setPendingChallenge] = useState<CloudAuthChallenge | null>(null);
  const [challengeValues, setChallengeValues] = useState<FormValues>({});
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  const providers = useMemo(() => providersQuery.data?.providers ?? [], [providersQuery.data]);
  const credentials = useMemo(
    () => credentialsQuery.data?.credentials ?? [],
    [credentialsQuery.data],
  );
  const selectedProvider = providers.find((provider) => provider.id === providerChoice) ?? null;

  const resetForm = () => {
    setDisplayName("");
    setFormValues({});
    setChallengeValues({});
    setPendingCredential(null);
    setPendingChallenge(null);
    setProviderChoice(null);
  };

  const closeModal = () => {
    if (createCredential.isPending || verifyChallenge.isPending) return;
    resetForm();
    setIsAddOpen(false);
  };

  const openModal = () => {
    setErrorMsg(null);
    setSuccessMsg(null);
    resetForm();
    setIsAddOpen(true);
  };

  const chooseProvider = (provider: CloudProvider) => {
    if (!provider.id) return;
    setProviderChoice(provider.id);
    setFormValues(fieldInitialValues(provider.form_fields));
  };

  const statusLabel = (status?: string) => {
    switch (status) {
      case "connected":
        return t("settings.cloud.status.connected");
      case "pending_challenge":
        return t("settings.cloud.status.pendingChallenge");
      case "error":
        return t("settings.cloud.status.error");
      case "disabled":
        return t("settings.cloud.status.disabled");
      default:
        return t("settings.cloud.status.unknown");
    }
  };

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selectedProvider?.id) return;
    setErrorMsg(null);
    setSuccessMsg(null);
    try {
      const result = await createCredential.mutateAsync({
        body: {
          display_name: displayName.trim() || undefined,
          provider: selectedProvider.id,
          inputs: formValues,
        },
      });
      const credential = result.credential;
      if (!credential) {
        throw new Error(t("settings.cloud.errors.emptyCredentialResponse"));
      }
      if (result.auth_status === "challenge_required" && result.challenge) {
        setPendingCredential(credential);
        setPendingChallenge(result.challenge);
        setChallengeValues(fieldInitialValues(result.challenge.fields));
        setSuccessMsg(t("settings.cloud.messages.challengeRequired"));
      } else {
        resetForm();
        setIsAddOpen(false);
        setSuccessMsg(t("settings.cloud.messages.connected"));
      }
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : t("settings.cloud.errors.createFailed"));
    }
  };

  const handleVerify = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!pendingCredential?.id) return;
    setErrorMsg(null);
    setSuccessMsg(null);
    try {
      await verifyChallenge.mutateAsync({
        params: { path: { id: pendingCredential.id } },
        body: { inputs: challengeValues },
      });
      resetForm();
      setIsAddOpen(false);
      setSuccessMsg(t("settings.cloud.messages.verified"));
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : t("settings.cloud.errors.verifyFailed"));
    }
  };

  const handleDisconnect = async (credential: CloudCredential) => {
    if (!confirm(t("settings.cloud.confirmDisconnect", { name: credential.display_name }))) return;
    setErrorMsg(null);
    setSuccessMsg(null);
    try {
      await disconnectCredential.mutateAsync({ params: { path: { id: credential.id ?? "" } } });
      setSuccessMsg(t("settings.cloud.messages.disconnected"));
    } catch (error) {
      setErrorMsg(
        error instanceof Error ? error.message : t("settings.cloud.errors.disconnectFailed"),
      );
    }
  };

  const handleReconnect = async (credential: CloudCredential) => {
    setErrorMsg(null);
    setSuccessMsg(null);
    try {
      const result = await reconnectCredential.mutateAsync({
        params: { path: { id: credential.id ?? "" } },
        body: { inputs: {} },
      });
      if (result.auth_status === "connected") {
        setSuccessMsg(t("settings.cloud.messages.reconnected"));
      } else if (result.auth_status === "password_required") {
        setErrorMsg(t("settings.cloud.errors.sessionExpired"));
      } else if (
        result.auth_status === "challenge_required" &&
        result.challenge &&
        result.credential
      ) {
        setPendingCredential(result.credential);
        setPendingChallenge(result.challenge);
        setChallengeValues(fieldInitialValues(result.challenge.fields));
        setSuccessMsg(t("settings.cloud.messages.challengeRequired"));
        setIsAddOpen(true);
      }
    } catch (error) {
      setErrorMsg(
        error instanceof Error ? error.message : t("settings.cloud.errors.reconnectFailed"),
      );
    }
  };

  const handleRemove = async (credential: CloudCredential) => {
    if (!confirm(t("settings.cloud.confirmRemove", { name: credential.display_name }))) return;
    setErrorMsg(null);
    setSuccessMsg(null);
    try {
      await removeCredential.mutateAsync({ params: { path: { id: credential.id ?? "" } } });
      setSuccessMsg(t("settings.cloud.messages.removed"));
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : t("settings.cloud.errors.removeFailed"));
    }
  };

  const renderField = (
    field: CloudProviderField,
    values: FormValues,
    setValues: (updater: (current: FormValues) => FormValues) => void,
    disabled: boolean,
  ) => {
    if (!field.name) return null;
    const id = `cloud-field-${field.name}`;
    const value = values[field.name] ?? "";
    const onChange = (next: string) =>
      setValues((current) => ({ ...current, [field.name!]: next }));

    return (
      <label key={field.name} className="form-control w-full" htmlFor={id}>
        <span className="label-text pb-1 text-sm font-medium">{field.label}</span>
        {field.type === "select" ? (
          <SettingsDropdown
            id={id}
            value={value}
            disabled={disabled}
            options={(field.options ?? []).map((option) => ({
              value: option.value ?? "",
              label: option.label ?? option.value ?? "",
            }))}
            onChange={onChange}
            ariaLabel={field.label}
            className="w-full"
            menuClassName="w-full"
          />
        ) : (
          <input
            id={id}
            type={field.type || "text"}
            className={`input input-bordered w-full ${field.type === "password" ? "font-mono" : ""}`}
            value={value}
            onChange={(event) => onChange(event.target.value)}
            placeholder={field.placeholder}
            autoComplete={field.autocomplete}
            disabled={disabled}
            required={field.required}
          />
        )}
        {field.help_text && (
          <span className="mt-1 text-xs text-base-content/55">{field.help_text}</span>
        )}
      </label>
    );
  };

  const busyLoading = credentialsQuery.isLoading || providersQuery.isLoading;

  return (
    <div className="w-full space-y-8 lg:space-y-10">
      {errorMsg && (
        <div className="flex items-center gap-2 rounded-xl bg-error/10 px-4 py-3 text-sm text-error">
          <AlertTriangleIcon className="size-4" />
          <span>{errorMsg}</span>
        </div>
      )}
      {successMsg && (
        <div className="flex items-center gap-2 rounded-xl bg-success/10 px-4 py-3 text-sm text-success">
          <CheckCircle2Icon className="size-4" />
          <span>{successMsg}</span>
        </div>
      )}

      <SettingsGroup
        title={t("settings.cloud.savedTitle")}
        description={t("settings.cloud.pageDescription", {
          defaultValue: "Manage connected cloud providers and credentials.",
        })}
      >
        {credentials.length === 0 && !credentialsQuery.isLoading ? (
          <SettingsBlock>
            <p className="text-center text-sm text-base-content/55">{t("settings.cloud.empty")}</p>
          </SettingsBlock>
        ) : (
          credentials.map((credential) => (
            <SettingsRow
              key={credential.id}
              align="start"
              icon={<KeyRoundIcon className="size-4" />}
              iconColor="bg-info text-info-content"
              label={credential.display_name}
              description={t("settings.cloud.credentialMeta", {
                provider: credential.provider_title ?? credential.provider,
                identity: credential.masked_identity,
              })}
              value={
                <span className={`badge badge-sm ${credentialStatusClass(credential.status)}`}>
                  {statusLabel(credential.status)}
                </span>
              }
              control={
                <div className="flex items-center gap-1">
                  {(credential.status === "disabled" || credential.status === "error") && (
                    <button
                      type="button"
                      className="btn btn-ghost btn-xs btn-circle text-success"
                      onClick={() => void handleReconnect(credential)}
                      disabled={reconnectCredential.isPending || !credential.id}
                      aria-label={t("settings.cloud.reconnect")}
                      title={t("settings.cloud.reconnect")}
                    >
                      <RefreshCwIcon size={16} />
                    </button>
                  )}
                  {credential.status === "connected" && (
                    <button
                      type="button"
                      className="btn btn-ghost btn-xs btn-circle text-warning"
                      onClick={() => void handleDisconnect(credential)}
                      disabled={disconnectCredential.isPending || !credential.id}
                      aria-label={t("settings.cloud.disconnect")}
                      title={t("settings.cloud.disconnect")}
                    >
                      <LogOutIcon size={16} />
                    </button>
                  )}
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs btn-circle text-error"
                    onClick={() => void handleRemove(credential)}
                    disabled={removeCredential.isPending || !credential.id}
                    aria-label={t("settings.cloud.remove")}
                    title={t("settings.cloud.remove")}
                  >
                    <Trash2Icon size={16} />
                  </button>
                </div>
              }
            />
          ))
        )}
        <SettingsRow
          icon={
            busyLoading ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <PlusIcon className="size-4" />
            )
          }
          iconColor="bg-primary text-primary-content"
          label={t("settings.cloud.addCredential")}
          onClick={providers.length === 0 ? undefined : openModal}
          chevron={providers.length > 0}
        />
      </SettingsGroup>

      {isAddOpen && (
        <div className="modal modal-open">
          <div className="modal-box max-w-lg rounded-2xl">
            <div className="mb-5 flex items-start justify-between gap-4">
              <div>
                <h3 className="text-lg font-semibold">
                  {pendingChallenge
                    ? (pendingChallenge.title ?? t("settings.cloud.verifyTitle"))
                    : selectedProvider
                      ? t("settings.cloud.providerFormTitle", { provider: selectedProvider.title })
                      : t("settings.cloud.providerTitle")}
                </h3>
                <p className="mt-1 text-sm text-base-content/65">
                  {pendingChallenge
                    ? (pendingChallenge.description ??
                      t("settings.cloud.verifyDescription", {
                        name: pendingCredential?.display_name,
                      }))
                    : selectedProvider
                      ? (selectedProvider.security_note ??
                        t("settings.cloud.providerFormDescription"))
                      : t("settings.cloud.providerDescription")}
                </p>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-sm btn-circle"
                onClick={closeModal}
                disabled={createCredential.isPending || verifyChallenge.isPending}
                aria-label={t("common.close", { defaultValue: "Close" })}
              >
                <XIcon size={18} />
              </button>
            </div>

            {!selectedProvider && !pendingChallenge && (
              <div className="grid gap-3">
                {providers.map((provider) => (
                  <button
                    key={provider.id}
                    type="button"
                    className="flex w-full items-center gap-3 rounded-xl border border-base-300 bg-base-100 p-4 text-left transition hover:border-primary disabled:opacity-60"
                    onClick={() => chooseProvider(provider)}
                    disabled={provider.status !== "enabled"}
                  >
                    <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      <CloudIcon size={20} />
                    </div>
                    <div className="min-w-0">
                      <div className="font-semibold">{provider.title}</div>
                      <div className="text-sm text-base-content/60">{provider.description}</div>
                    </div>
                  </button>
                ))}
              </div>
            )}

            {selectedProvider && !pendingChallenge && (
              <form onSubmit={handleCreate} className="space-y-4">
                <label className="form-control w-full" htmlFor="cloud-display-name">
                  <span className="label-text pb-1 text-sm font-medium">
                    {t("settings.cloud.fields.label")}
                  </span>
                  <input
                    id="cloud-display-name"
                    type="text"
                    className="input input-bordered w-full"
                    value={displayName}
                    onChange={(event) => setDisplayName(event.target.value)}
                    placeholder={t("settings.cloud.placeholders.label")}
                    disabled={createCredential.isPending}
                  />
                </label>
                {(selectedProvider.form_fields ?? []).map((field) =>
                  renderField(field, formValues, setFormValues, createCredential.isPending),
                )}
                <div className="modal-action">
                  <button
                    type="button"
                    className="btn btn-ghost"
                    onClick={() => setProviderChoice(null)}
                    disabled={createCredential.isPending}
                  >
                    {t("common.back", { defaultValue: "Back" })}
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary gap-2"
                    disabled={
                      !requiredFieldsFilled(selectedProvider.form_fields, formValues) ||
                      createCredential.isPending
                    }
                  >
                    {createCredential.isPending ? (
                      <span className="loading loading-spinner loading-xs" />
                    ) : (
                      <ShieldCheckIcon size={16} />
                    )}
                    {t("settings.cloud.connectCredential")}
                  </button>
                </div>
              </form>
            )}

            {pendingChallenge && (
              <form onSubmit={handleVerify} className="space-y-4">
                {(pendingChallenge.fields ?? []).map((field) =>
                  renderField(
                    field,
                    challengeValues,
                    setChallengeValues,
                    verifyChallenge.isPending,
                  ),
                )}
                <div className="modal-action">
                  <button
                    type="button"
                    className="btn btn-ghost"
                    disabled={verifyChallenge.isPending}
                    onClick={closeModal}
                  >
                    {t("common.cancel", { defaultValue: "Cancel" })}
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary"
                    disabled={
                      !requiredFieldsFilled(pendingChallenge.fields, challengeValues) ||
                      verifyChallenge.isPending
                    }
                  >
                    {verifyChallenge.isPending && (
                      <span className="loading loading-spinner loading-xs" />
                    )}
                    {t("settings.cloud.verify")}
                  </button>
                </div>
              </form>
            )}
          </div>
          <button
            type="button"
            className="modal-backdrop"
            aria-label={t("common.close", { defaultValue: "Close" })}
            onClick={closeModal}
          />
        </div>
      )}
    </div>
  );
}
