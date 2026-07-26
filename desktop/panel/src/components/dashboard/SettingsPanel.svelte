<script lang="ts">
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { refreshState, store } from "../../lib/store.svelte.ts";
  import type { NetworkMode } from "../../lib/types.ts";
  import RegionSelect from "../shared/RegionSelect.svelte";

  let region = $state(store.data!.region === "cn" ? "cn" : "other");
  const initial = store.data!.network;
  let mode = $state<NetworkMode>(initial.mode);
  let primaryOrigin = $state(initial.primaryOrigin);
  let listen = $state(initial.listen);
  let proxyLocation = $state<"same_host" | "remote">(
    initial.trustedProxyCIDRs.some((cidr) => cidr !== "127.0.0.1/32" && cidr !== "::1/128")
      ? "remote"
      : "same_host",
  );
  let trustedCIDRs = $state(initial.trustedProxyCIDRs.join("\n"));
  let acceptLANWarning = $state(initial.lanWarningAcceptedVersion >= 1);
  let saving = $state(false);
  let message = $state("");
  let error = $state("");

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

  async function saveNetwork(): Promise<void> {
    saving = true;
    message = "";
    error = "";
    try {
      const result = await api.saveNetwork({
        mode,
        primaryOrigin,
        listen,
        proxyLocation,
        trustedProxyCIDRs: trustedCIDRs
          .split(/\s+/)
          .map((value) => value.trim())
          .filter(Boolean),
        acceptLANWarning,
      });
      message = result.rpIDChanged ? t("networkRPWarning") : t("networkSaved");
      await refreshState();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      saving = false;
    }
  }
</script>

<div class="flex flex-col gap-2.5 rounded-[10px] border border-line bg-raised px-4 py-3.5">
  <span class="text-[13.5px] font-semibold">{t("settings")}</span>
  <div class="flex flex-wrap items-center gap-3.5">
    <span class="text-xs font-semibold tracking-wide text-muted uppercase">{t("regionLabel")}</span>
    <RegionSelect bind:region showHint={false} onchange={(r) => void api.saveRegion(r)} />
  </div>
  <p class="m-0 text-xs text-muted">
    {region === "cn" ? t("regionHintCn") : t("regionHintOther")}
  </p>

  <div class="my-1 h-px bg-line"></div>
  <span class="text-xs font-semibold tracking-wide text-muted uppercase">{t("networkAccess")}</span>
  <select class="select select-sm w-full" bind:value={mode}>
    <option value="local">{t("networkLocal")}</option>
    <option value="lan_http">{t("networkLAN")}</option>
    <option value="external_https">{t("networkExternal")}</option>
  </select>

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
      <input class="checkbox checkbox-warning checkbox-sm" type="checkbox" bind:checked={acceptLANWarning} />
      <span>{t("networkAcceptLAN")}</span>
    </label>
  {:else}
    <label class="flex flex-col gap-1 text-xs">
      <span class="font-semibold">{t("networkOrigin")}</span>
      <input class="input input-sm w-full" type="url" bind:value={primaryOrigin} placeholder="https://photos.example.com" />
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
      <input class="input input-sm w-full" bind:value={listen} placeholder={proxyLocation === "same_host" ? "127.0.0.1:6680" : "0.0.0.0:6680"} />
    </label>
    {#if proxyLocation === "remote"}
      <label class="flex flex-col gap-1 text-xs">
        <span class="font-semibold">{t("networkTrustedCIDRs")}</span>
        <textarea class="textarea textarea-sm w-full" rows="2" bind:value={trustedCIDRs} placeholder="192.168.1.10/32"></textarea>
      </label>
    {/if}
    <p class="m-0 text-xs text-muted">{t("networkExternalHint")}</p>
    {#if hostnameWillChange}
      <div class="alert alert-warning alert-soft text-xs"><span>{t("networkRPWarning")}</span></div>
    {/if}
  {/if}

  {#if error}
    <div class="alert alert-error alert-soft text-xs"><span>{error}</span></div>
  {:else if message}
    <div class="alert alert-success alert-soft text-xs"><span>{message}</span></div>
  {/if}
  <button
    class="btn btn-primary btn-sm self-start"
    disabled={saving || (mode === "lan_http" && !acceptLANWarning)}
    onclick={() => void saveNetwork()}
  >
    {t("networkSave")}
  </button>
</div>
