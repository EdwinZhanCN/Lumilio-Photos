<script lang="ts">
  import { untrack } from "svelte";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { refreshState, store } from "../../lib/store.svelte.ts";
  import type { Validation } from "../../lib/types.ts";
  import BackendPicker from "../shared/BackendPicker.svelte";
  import PathPicker from "../shared/PathPicker.svelte";
  import PresetPicker from "../shared/PresetPicker.svelte";

  let {
    open,
    session,
    dirty = $bindable(false),
    saving = $bindable(false),
    confirmingMove = $bindable(false),
  }: {
    open: boolean;
    session: number;
    dirty?: boolean;
    saving?: boolean;
    confirmingMove?: boolean;
  } = $props();

  const data = $derived(store.data!);
  let profile = $state("");
  let preset = $state("");
  let cacheDir = $state("");
  let cacheValidation = $state<Validation | null>(null);
  let originalProfile = "";
  let originalPreset = "";
  let originalCacheDir = "";
  let error = $state("");

  function seed(): void {
    const d = store.data!;
    const l = d.lumen;
    profile = l.profile || (d.backends.find((b) => b.recommended) ?? d.backends[0])?.profile || "";
    preset = l.preset || d.recommendedPreset;
    cacheDir = l.cacheDir;
    cacheValidation = d.cacheValidation;
    originalProfile = profile;
    originalPreset = preset;
    originalCacheDir = cacheDir;
    confirmingMove = false;
    error = "";
  }

  $effect(() => {
    if (!open) return;
    void session;
    untrack(seed);
  });

  $effect(() => {
    dirty =
      profile !== originalProfile || preset !== originalPreset || cacheDir !== originalCacheDir;
    if (cacheDir === originalCacheDir) confirmingMove = false;
  });

  async function pickCache(): Promise<void> {
    const result = await api.pickCache();
    if (!result.cancelled && result.path) {
      cacheDir = result.path;
      cacheValidation = result.validation ?? null;
    }
  }

  export async function save(): Promise<boolean> {
    if (cacheDir !== originalCacheDir && !confirmingMove) {
      confirmingMove = true;
      return false;
    }
    saving = true;
    error = "";
    try {
      const backend = data.backends.find((choice) => choice.profile === profile);
      await api.lumenSave({
        preset,
        backend: backend?.name ?? "",
        profile,
        cacheDir,
      });
      seed();
      setTimeout(() => void refreshState(), 350);
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
    <h3 class="m-0 text-sm font-semibold">{t("lumenSettings")}</h3>
    <p class="m-0 mt-1 text-xs text-muted">{t("lumenSettingsHint")}</p>
  </div>
  <div class="text-xs font-semibold tracking-wide text-muted uppercase">{t("s2title")}</div>
  <BackendPicker backends={data.backends} bind:profile />
  <div class="text-xs font-semibold tracking-wide text-muted uppercase">{t("s3title")}</div>
  <PresetPicker presets={data.presets} recommended={data.recommendedPreset} bind:preset />
  <PathPicker
    label={t("cacheLabel")}
    path={cacheDir}
    validation={cacheValidation}
    onpick={pickCache}
  />
  {#if confirmingMove}
    <div role="alert" class="alert alert-warning alert-soft text-xs">
      <span>{t("cacheMoveWarning")}</span>
    </div>
  {/if}
  {#if error}
    <div role="alert" class="alert alert-error alert-soft text-xs"><span>{error}</span></div>
  {/if}
</div>
