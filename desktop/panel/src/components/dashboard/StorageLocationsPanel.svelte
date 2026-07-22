<script lang="ts">
  import { api, PanelAPIError } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import type {
    RepositoryIdentityConflict,
    StorageLocation,
    StorageLocationIdentityConflict,
  } from "../../lib/types.ts";

  let locations = $state<StorageLocation[]>([]);
  let loading = $state(true);
  let busy = $state(false);
  let message = $state("");
  let error = $state("");
  let conflict = $state<RepositoryIdentityConflict | null>(null);
  let locationConflict = $state<StorageLocationIdentityConflict | null>(null);

  async function load() {
    loading = true;
    try {
      locations = (await api.storageLocations()).locations;
      error = "";
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      loading = false;
    }
  }

  async function addLocation() {
    busy = true;
    message = "";
    error = "";
    locationConflict = null;
    try {
      const result = await api.pickStorageLocation();
      if (!result.cancelled) {
        message = result.warnings?.[0] ?? t("storageLocationAdded");
        await load();
      }
    } catch (cause) {
      if (cause instanceof PanelAPIError && cause.status === 409) {
        const payload = cause.payload as { conflict?: StorageLocationIdentityConflict };
        if (payload.conflict) locationConflict = payload.conflict;
      } else {
        error = cause instanceof Error ? cause.message : String(cause);
      }
    } finally {
      busy = false;
    }
  }

  async function reconnectLocation() {
    if (!locationConflict) return;
    busy = true;
    error = "";
    try {
      await api.resolveStorageLocationConflict(locationConflict);
      message = t("storageLocationReconnected");
      locationConflict = null;
      await load();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      busy = false;
    }
  }

  async function attachRepository() {
    busy = true;
    message = "";
    error = "";
    conflict = null;
    try {
      const result = await api.attachRepository();
      if (!result.cancelled && result.repository) {
        message = t("repositoryAttached", { name: result.repository.name });
      }
    } catch (cause) {
      if (cause instanceof PanelAPIError && cause.status === 409) {
        const payload = cause.payload as { conflict?: RepositoryIdentityConflict };
        if (payload.conflict) conflict = payload.conflict;
      } else {
        error = cause instanceof Error ? cause.message : String(cause);
      }
    } finally {
      busy = false;
    }
  }

  async function resolveConflict(action: "relocate" | "copy") {
    if (!conflict) return;
    busy = true;
    error = "";
    try {
      const result = await api.resolveRepositoryConflict(action, conflict);
      message = t(action === "relocate" ? "repositoryRelocated" : "repositoryCopyAttached", {
        name: result.repository.name,
      });
      conflict = null;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      busy = false;
    }
  }

  async function removeLocation(id: string) {
    busy = true;
    message = "";
    error = "";
    try {
      await api.removeStorageLocation(id);
      await load();
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      busy = false;
    }
  }

  $effect(() => {
    void load();
  });
</script>

<section class="rounded-[10px] border border-line bg-raised px-4 py-3.5">
  <div class="flex flex-wrap items-start justify-between gap-3">
    <div>
      <h2 class="m-0 text-[13.5px] font-semibold">{t("storageLocations")}</h2>
      <p class="m-0 mt-1 text-xs text-muted">{t("storageLocationsHint")}</p>
    </div>
    <div class="flex gap-2">
      <button class="btn btn-sm" disabled={busy} onclick={() => void attachRepository()}>
        {t("attachRepository")}
      </button>
      <button class="btn btn-primary btn-sm" disabled={busy} onclick={() => void addLocation()}>
        {t("addStorageLocation")}
      </button>
    </div>
  </div>

  {#if loading}
    <div class="mt-3 flex items-center gap-2 text-xs text-muted">
      <span class="loading loading-spinner loading-xs"></span>{t("loadingStorageLocations")}
    </div>
  {:else}
    <ul class="list mt-3 rounded-lg border border-line bg-surface p-0">
      {#each locations as location (location.id)}
        <li class="list-row items-center gap-3 border-b border-line px-3 py-2.5 last:border-b-0">
          <div class="min-w-0 list-col-grow">
            <div class="flex items-center gap-2">
              <span class="truncate text-xs font-semibold">{location.name}</span>
              <span
                class:badge-success={location.status === "active"}
                class:badge-warning={location.status === "offline"}
                class:badge-error={location.status === "error"}
                class="badge badge-soft badge-xs"
              >
                {location.status === "active"
                  ? t("locationActive")
                  : location.status === "offline"
                    ? t("locationOffline")
                    : t("locationError")}
              </span>
              {#if location.kind === "default"}
                <span class="badge badge-ghost badge-xs">{t("locationDefault")}</span>
              {/if}
            </div>
            <div class="mt-0.5 truncate font-mono text-[11px] text-muted" title={location.path}>
              {location.path}
            </div>
          </div>
          <button class="btn btn-ghost btn-xs" onclick={() => void api.openPath(location.path)}>
            {t("reveal")}
          </button>
          {#if location.kind === "external"}
            <button
              class="btn btn-ghost btn-xs text-error"
              disabled={busy}
              onclick={() => void removeLocation(location.id)}
            >
              {t("remove")}
            </button>
          {/if}
        </li>
      {/each}
    </ul>
  {/if}

  {#if locationConflict}
    <div role="alert" class="alert alert-warning alert-soft mt-3 flex-col items-start gap-2 text-xs">
      <strong>{t("storageLocationConflictTitle")}</strong>
      <p class="m-0">{t("storageLocationConflictBody")}</p>
      <div class="grid w-full gap-1 font-mono text-[11px]">
        <span class="truncate" title={locationConflict.registeredPath}>{locationConflict.registeredPath}</span>
        <span class="truncate" title={locationConflict.requestedPath}>{locationConflict.requestedPath}</span>
      </div>
      <button class="btn btn-warning btn-xs" disabled={busy} onclick={() => void reconnectLocation()}>
        {t("storageLocationReconnectHere")}
      </button>
    </div>
  {/if}

  {#if conflict}
    <div role="alert" class="alert alert-warning alert-soft mt-3 flex-col items-start gap-2 text-xs">
      <strong>{t("repositoryConflictTitle")}</strong>
      <p class="m-0">{t("repositoryConflictBody")}</p>
      <div class="grid w-full gap-1 font-mono text-[11px]">
        <span class="truncate" title={conflict.registeredPath}>{conflict.registeredPath}</span>
        <span class="truncate" title={conflict.requestedPath}>{conflict.requestedPath}</span>
      </div>
      <div class="flex flex-wrap gap-2">
        <button class="btn btn-warning btn-xs" disabled={busy} onclick={() => void resolveConflict("relocate")}>
          {t("repositoryTreatAsMoved")}
        </button>
        <button class="btn btn-ghost btn-xs" disabled={busy} onclick={() => void resolveConflict("copy")}>
          {t("repositoryRegisterCopy")}
        </button>
      </div>
    </div>
  {/if}

  {#if message}
    <div role="status" class="alert alert-success alert-soft mt-3 py-2 text-xs">{message}</div>
  {/if}
  {#if error}
    <div role="alert" class="alert alert-error alert-soft mt-3 py-2 text-xs">{error}</div>
  {/if}
</section>
