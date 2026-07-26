<script lang="ts">
  import { untrack } from "svelte";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import {
    acceptRuntimeValidation,
    loadRuntimeDraft,
    runtimeDraft,
  } from "../../lib/runtime-draft.svelte.ts";
  import { refreshState, store } from "../../lib/store.svelte.ts";
  import type { NetworkInfo, NetworkMode } from "../../lib/types.ts";

  let {
    open,
    session,
    dirty = $bindable(false),
    saving = $bindable(false),
    canSave = $bindable(true),
  }: {
    open: boolean;
    session: number;
    dirty?: boolean;
    saving?: boolean;
    canSave?: boolean;
  } = $props();

  let initial = $state<NetworkInfo>(store.data!.network);
  let mode = $state<NetworkMode>(store.data!.network.mode);
  let primaryOrigin = $state(store.data!.network.primaryOrigin);
  let listen = $state(store.data!.network.listen);
  let proxyLocation = $state<"same_host" | "remote">("same_host");
  let trustedCIDRs = $state("");
  let acceptLANWarning = $state(false);
  let message = $state("");
  let error = $state("");

  function proxyLocationFor(network: NetworkInfo): "same_host" | "remote" {
    return network.trustedProxyCIDRs.some(
      (cidr) => cidr !== "127.0.0.1/32" && cidr !== "::1/128",
    )
      ? "remote"
      : "same_host";
  }

  function seed(): void {
    initial = structuredClone(store.data!.network);
    mode = initial.mode;
    primaryOrigin = initial.primaryOrigin;
    listen = initial.listen;
    proxyLocation = proxyLocationFor(initial);
    trustedCIDRs = initial.trustedProxyCIDRs.join("\n");
    acceptLANWarning = initial.lanWarningAcceptedVersion >= 1;
    message = "";
    error = "";
  }

  $effect(() => {
    if (!open) return;
    void session;
    untrack(seed);
  });

  function hostname(origin: string): string {
    try {
      return new URL(origin).hostname;
    } catch {
      return "";
    }
  }

  const hostnameWillChange = $derived(
    mode === "external_https" &&
      hostname(primaryOrigin) !== "" &&
      hostname(primaryOrigin) !== hostname(initial.primaryOrigin),
  );

  $effect(() => {
    dirty =
      mode !== initial.mode ||
      primaryOrigin !== initial.primaryOrigin ||
      listen !== initial.listen ||
      proxyLocation !== proxyLocationFor(initial) ||
      trustedCIDRs !== initial.trustedProxyCIDRs.join("\n") ||
      acceptLANWarning !== (initial.lanWarningAcceptedVersion >= 1);
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
          primaryOrigin,
          listen,
          proxyLocation,
          trustedProxyCIDRs: trustedCIDRs
            .split(/\s+/)
            .map((value) => value.trim())
            .filter(Boolean),
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
      <option value="external_https">{t("networkExternal")}</option>
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
  {:else}
    <label class="flex flex-col gap-1 text-xs">
      <span class="font-semibold">{t("networkOrigin")}</span>
      <input
        class="input input-sm w-full"
        type="url"
        bind:value={primaryOrigin}
        placeholder="https://photos.example.com"
      />
    </label>
    <label class="flex flex-col gap-1 text-xs">
      <span class="font-semibold">{t("networkProxyLocation")}</span>
      <select class="select select-sm w-full" bind:value={proxyLocation}>
        <option value="same_host">{t("networkProxySameHost")}</option>
        <option value="remote">{t("networkProxyRemote")}</option>
      </select>
    </label>
    <label class="flex flex-col gap-1 text-xs">
      <span class="font-semibold">{t("networkListen")}</span>
      <input
        class="input input-sm w-full"
        bind:value={listen}
        placeholder={proxyLocation === "same_host" ? "127.0.0.1:6680" : "0.0.0.0:6680"}
      />
    </label>
    {#if proxyLocation === "remote"}
      <label class="flex flex-col gap-1 text-xs">
        <span class="font-semibold">{t("networkTrustedCIDRs")}</span>
        <textarea
          class="textarea textarea-sm w-full"
          rows="2"
          bind:value={trustedCIDRs}
          placeholder="192.168.1.10/32"
        ></textarea>
      </label>
    {/if}
    <p class="m-0 text-xs text-muted">{t("networkExternalHint")}</p>
    {#if hostnameWillChange}
      <div class="alert alert-warning alert-soft text-xs"><span>{t("networkRPWarning")}</span></div>
    {/if}
  {/if}

  {#if error}
    <div role="alert" class="alert alert-error alert-soft text-xs"><span>{error}</span></div>
  {:else if message}
    <div role="status" class="alert alert-success alert-soft text-xs"><span>{message}</span></div>
  {/if}
</div>
