<script lang="ts">
  import { untrack } from "svelte";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import {
    acceptRuntimeValidation,
    loadRuntimeDraft,
    resolvedRuntimeDraftNetwork,
    runtimeDraft,
  } from "../../lib/runtime-draft.svelte.ts";
  import { refreshState, store } from "../../lib/store.svelte.ts";
  import type { NetworkInfo, NetworkMode } from "../../lib/types.ts";
  import { networkDraftFromResolvedState } from "./network-draft.ts";

  let {
    open,
    active,
    session,
    dirty = $bindable(false),
    saving = $bindable(false),
    canSave = $bindable(true),
  }: {
    open: boolean;
    active: boolean;
    session: number;
    dirty?: boolean;
    saving?: boolean;
    canSave?: boolean;
  } = $props();

  function currentNetwork(): NetworkInfo {
    return networkDraftFromResolvedState(store.data!.runtime.network, store.data!.networkHost);
  }

  const initialNetwork = currentNetwork();
  let initial = $state<NetworkInfo>(initialNetwork);
  let mode = $state<NetworkMode>(initialNetwork.mode);
  let acceptLANWarning = $state(false);
  let message = $state("");
  let error = $state("");
  let candidateSyncToken = 0;

  function seed(network = currentNetwork()): void {
    initial = structuredClone(network);
    mode = initial.mode;
    acceptLANWarning = initial.lanWarningAcceptedVersion >= 1;
    message = "";
    error = "";
  }

  function fieldsDirty(): boolean {
    return mode !== initial.mode ||
      acceptLANWarning !== (initial.lanWarningAcceptedVersion >= 1);
  }

  async function syncResolvedCandidateNetwork(): Promise<void> {
    if (!runtimeDraft.view || fieldsDirty()) return;
    const token = ++candidateSyncToken;
    const candidate = runtimeDraft.candidateToml;
    let resolved = resolvedRuntimeDraftNetwork();
    if (!resolved) {
      try {
        const validation = await api.validateRuntimeConfig({
          baseFingerprint: runtimeDraft.view.baseFingerprint,
          toml: candidate,
        });
        if (token !== candidateSyncToken || candidate !== runtimeDraft.candidateToml) return;
        acceptRuntimeValidation(validation);
        if (!validation.valid) {
          error = validation.issues.map((issue) => issue.message).join("; ");
          return;
        }
        resolved = validation.network;
      } catch (cause) {
        if (token === candidateSyncToken) {
          error = cause instanceof Error ? cause.message : String(cause);
        }
        return;
      }
    }
    if (token !== candidateSyncToken || !active || fieldsDirty()) return;
    seed(networkDraftFromResolvedState(resolved, store.data!.networkHost));
  }

  $effect(() => {
    if (!open) return;
    void session;
    untrack(seed);
  });

  $effect(() => {
    if (!open || !active || !runtimeDraft.view) return;
    void runtimeDraft.candidateToml;
    untrack(() => void syncResolvedCandidateNetwork());
  });

  $effect(() => {
    dirty = fieldsDirty();
    canSave = mode !== "lan_http" || acceptLANWarning;
  });

  export async function save(): Promise<boolean> {
    saving = true;
    message = "";
    error = "";
    try {
      if (!runtimeDraft.view) await loadRuntimeDraft(session);
      if (!runtimeDraft.view) {
        throw new Error(runtimeDraft.error || t("runtimeConfigurationUnavailable"));
      }
      const validation = await api.patchRuntimeNetwork({
        baseFingerprint: runtimeDraft.view.baseFingerprint,
        toml: runtimeDraft.candidateToml,
        network: {
          mode,
          acceptLANWarning,
        },
      });
      acceptRuntimeValidation(validation);
      if (!validation.valid) {
        throw new Error(validation.issues.map((issue) => issue.message).join("; "));
      }
      await api.applyRuntimeConfig({
        baseFingerprint: runtimeDraft.view.baseFingerprint,
        toml: validation.candidateToml,
        acceptLANWarning,
      });
      message = t("networkApplyAccepted");
      setTimeout(() => void refreshState(), 250);
      return true;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
      return false;
    } finally {
      saving = false;
    }
  }
</script>

<div class="flex flex-col gap-3.5">
  <div>
    <h3 class="m-0 text-sm font-semibold">{t("serverSettings")}</h3>
    <p class="m-0 mt-1 text-xs text-muted">{t("serverSettingsHint")}</p>
  </div>

  <label class="flex flex-col gap-1.5 text-xs font-semibold">
    <span>{t("networkAccess")}</span>
    <select class="select select-sm w-full" bind:value={mode}>
      <option value="local">{t("networkLocal")}</option>
      <option value="lan_http">{t("networkLAN")}</option>
    </select>
  </label>

  {#if mode === "local"}
    <p class="m-0 text-xs text-muted">{t("networkLocalHint")}</p>
  {:else if mode === "lan_http"}
    <div class="alert alert-warning alert-soft text-xs">
      <span>{t("networkLANWarning")}</span>
    </div>
    <p class="m-0 text-xs text-muted">
      {t("networkLANAddresses", {
        addresses: initial.lanAddresses.length > 0 ? initial.lanAddresses.join(", ") : "—",
      })}
    </p>
    <p class="m-0 text-xs text-muted">{t("networkFirewall")}</p>
    <label class="flex cursor-pointer items-center gap-2 text-xs">
      <input
        class="checkbox checkbox-warning checkbox-sm"
        type="checkbox"
        bind:checked={acceptLANWarning}
      />
      <span>{t("networkAcceptLAN")}</span>
    </label>
  {/if}

  {#if error}
    <div role="alert" class="alert alert-error alert-soft text-xs"><span>{error}</span></div>
  {:else if message}
    <div role="status" class="alert alert-success alert-soft text-xs"><span>{message}</span></div>
  {/if}
</div>
