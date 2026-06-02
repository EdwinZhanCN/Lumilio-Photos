import { useMemo, useState, type FormEvent } from "react";
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  CloudIcon,
  KeyRoundIcon,
  LogOutIcon,
  PlusIcon,
  ShieldCheckIcon,
  XIcon,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useCloudCredentials,
  useCloudProviders,
  useCreateCloudCredential,
  useDisableCloudCredential,
  useVerifyCloudCredentialChallenge,
  type CloudAuthChallenge,
  type CloudCredential,
  type CloudProvider,
  type CloudProviderField,
} from "../../hooks/useCloudSync";

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

export default function CloudSettings() {
  const { t } = useI18n();
  const providersQuery = useCloudProviders();
  const credentialsQuery = useCloudCredentials();
  const createCredential = useCreateCloudCredential();
  const verifyChallenge = useVerifyCloudCredentialChallenge();
  const disableCredential = useDisableCloudCredential();

  const [isAddOpen, setIsAddOpen] = useState(false);
  const [providerChoice, setProviderChoice] = useState<string | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [formValues, setFormValues] = useState<FormValues>({});
  const [pendingCredential, setPendingCredential] = useState<CloudCredential | null>(null);
  const [pendingChallenge, setPendingChallenge] = useState<CloudAuthChallenge | null>(null);
  const [challengeValues, setChallengeValues] = useState<FormValues>({});
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  const providers = useMemo(
    () => providersQuery.data?.data?.providers ?? [],
    [providersQuery.data],
  );
  const credentials = useMemo(
    () => credentialsQuery.data?.data?.credentials ?? [],
    [credentialsQuery.data],
  );
  const selectedProvider = providers.find((provider) => provider.id === providerChoice) ?? null;
  const connectedCount = credentials.filter((item) => item.status === "connected").length;

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
      const credential = result.data?.credential;
      if (!credential) {
        throw new Error(t("settings.cloud.errors.emptyCredentialResponse"));
      }
      if (result.data?.auth_status === "challenge_required" && result.data.challenge) {
        setPendingCredential(credential);
        setPendingChallenge(result.data.challenge);
        setChallengeValues(fieldInitialValues(result.data.challenge.fields));
        setSuccessMsg(t("settings.cloud.messages.challengeRequired"));
      } else {
        resetForm();
        setIsAddOpen(false);
        setSuccessMsg(t("settings.cloud.messages.connected"));
      }
    } catch (error) {
      setErrorMsg(
        error instanceof Error ? error.message : t("settings.cloud.errors.createFailed"),
      );
    }
  };

  const handleVerify = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!pendingCredential?.id) return;
    setErrorMsg(null);
    setSuccessMsg(null);

    try {
      await verifyChallenge.mutateAsync({
        params: {
          path: {
            id: pendingCredential.id,
          },
        },
        body: {
          inputs: challengeValues,
        },
      });
      resetForm();
      setIsAddOpen(false);
      setSuccessMsg(t("settings.cloud.messages.verified"));
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : t("settings.cloud.errors.verifyFailed"));
    }
  };

  const handleDisable = async (credential: CloudCredential) => {
    if (
      !confirm(
        t("settings.cloud.confirmDisable", {
          name: credential.display_name,
        }),
      )
    ) {
      return;
    }
    setErrorMsg(null);
    setSuccessMsg(null);

    try {
      await disableCredential.mutateAsync({
        params: {
          path: {
            id: credential.id ?? "",
          },
        },
      });
      setSuccessMsg(t("settings.cloud.messages.disabled"));
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : t("settings.cloud.errors.disableFailed"));
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
    const onChange = (next: string) => {
      setValues((current) => ({ ...current, [field.name!]: next }));
    };

    return (
      <label key={field.name} className="form-control w-full" htmlFor={id}>
        <span className="label-text pb-1 text-sm font-medium">{field.label}</span>
        {field.type === "select" ? (
          <select
            id={id}
            className="select select-bordered w-full"
            value={value}
            onChange={(event) => onChange(event.target.value)}
            disabled={disabled}
            required={field.required}
          >
            {(field.options ?? []).map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
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
        {field.help_text && <span className="mt-1 text-xs text-base-content/55">{field.help_text}</span>}
      </label>
    );
  };

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <div className="flex items-center gap-2">
          <div className="rounded-lg bg-primary/10 p-2 text-primary">
            <CloudIcon className="size-6" />
          </div>
          <h2 className="text-2xl font-bold">{t("settings.cloud.title")}</h2>
        </div>
        <p className="text-sm text-base-content/65">{t("settings.cloud.description")}</p>
      </header>

      {errorMsg && (
        <div className="alert alert-error rounded-lg">
          <AlertTriangleIcon className="size-5" />
          <span>{errorMsg}</span>
        </div>
      )}
      {successMsg && (
        <div className="alert alert-success rounded-lg">
          <CheckCircle2Icon className="size-5" />
          <span>{successMsg}</span>
        </div>
      )}

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h3 className="text-base font-semibold">{t("settings.cloud.savedTitle")}</h3>
            <p className="text-sm text-base-content/60">
              {t("settings.cloud.connectedAccounts", { count: connectedCount })}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {(credentialsQuery.isLoading || providersQuery.isLoading) && (
              <span className="loading loading-spinner loading-sm" />
            )}
            <button
              type="button"
              className="btn btn-primary btn-sm btn-circle"
              onClick={openModal}
              aria-label={t("settings.cloud.addCredential")}
              title={t("settings.cloud.addCredential")}
              disabled={providers.length === 0}
            >
              <PlusIcon size={18} />
            </button>
          </div>
        </div>

        {credentials.length === 0 && !credentialsQuery.isLoading ? (
          <div className="rounded-lg border border-base-300 px-4 py-8 text-center text-sm text-base-content/60">
            {t("settings.cloud.empty")}
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {credentials.map((credential) => (
              <article
                key={credential.id}
                className="rounded-lg border border-base-300 bg-base-100 p-4 shadow-sm"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex min-w-0 items-start gap-3">
                    <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      <KeyRoundIcon size={20} />
                    </div>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <h4 className="truncate text-sm font-semibold">
                          {credential.display_name}
                        </h4>
                        <span
                          className={`badge badge-sm ${credentialStatusClass(credential.status)}`}
                        >
                          {statusLabel(credential.status)}
                        </span>
                      </div>
                      <p className="mt-1 text-xs text-base-content/60">
                        {t("settings.cloud.credentialMeta", {
                          provider: credential.provider_title ?? credential.provider,
                          identity: credential.masked_identity,
                        })}
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs btn-circle text-error"
                    onClick={() => void handleDisable(credential)}
                    disabled={disableCredential.isPending || !credential.id}
                    aria-label={t("settings.cloud.disable")}
                    title={t("settings.cloud.disable")}
                  >
                    <LogOutIcon size={16} />
                  </button>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>

      {isAddOpen && (
        <div className="modal modal-open">
          <div className="modal-box max-w-lg rounded-lg">
            <div className="mb-5 flex items-start justify-between gap-4">
              <div>
                <h3 className="text-lg font-semibold">
                  {pendingChallenge
                    ? (pendingChallenge.title ?? t("settings.cloud.verifyTitle"))
                    : selectedProvider
                      ? t("settings.cloud.providerFormTitle", {
                          provider: selectedProvider.title,
                        })
                      : t("settings.cloud.providerTitle")}
                </h3>
                <p className="mt-1 text-sm text-base-content/65">
                  {pendingChallenge
                    ? (pendingChallenge.description ??
                      t("settings.cloud.verifyDescription", {
                        name: pendingCredential?.display_name,
                      }))
                    : selectedProvider
                      ? (selectedProvider.security_note ?? t("settings.cloud.providerFormDescription"))
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
                    className="flex w-full items-center gap-3 rounded-lg border border-base-300 bg-base-100 p-4 text-left transition hover:border-primary disabled:opacity-60"
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
                  renderField(field, challengeValues, setChallengeValues, verifyChallenge.isPending),
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
                    {verifyChallenge.isPending && <span className="loading loading-spinner loading-xs" />}
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
