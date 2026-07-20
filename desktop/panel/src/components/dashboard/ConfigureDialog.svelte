<script lang="ts">
  import { untrack } from "svelte";
  import { Dialog } from "bits-ui";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { refreshState, store } from "../../lib/store.svelte.ts";
  import type { Validation } from "../../lib/types.ts";
  import BackendPicker from "../shared/BackendPicker.svelte";
  import PathPicker from "../shared/PathPicker.svelte";
  import PresetPicker from "../shared/PresetPicker.svelte";

  let { open = $bindable() }: { open: boolean } = $props();

  const data = $derived(store.data!);

  let profile = $state("");
  let preset = $state("");
  let cacheDir = $state("");
  let cacheValidation = $state<Validation | null>(null);
  let originalCacheDir = "";
  let confirmingMove = $state(false);
  let saving = $state(false);
  let error = $state("");

  $effect(() => {
    if (!open) return;
    // Re-seed from current settings each time the dialog opens. untrack keeps
    // this effect keyed on `open` only, so background status polls cannot
    // clobber in-progress edits.
    untrack(() => {
      const d = store.data!;
      const l = d.lumen;
      profile =
        l.profile || (d.backends.find((b) => b.recommended) ?? d.backends[0])?.profile || "";
      preset = l.preset || d.recommendedPreset;
      cacheDir = l.cacheDir;
      cacheValidation = d.cacheValidation;
      originalCacheDir = l.cacheDir;
      confirmingMove = false;
      error = "";
    });
  });

  async function pickCache() {
    const r = await api.pickCache();
    if (!r.cancelled && r.path) {
      cacheDir = r.path;
      cacheValidation = r.validation ?? null;
    }
  }

  function requestSave() {
    // Moving the model cache is slow and pauses the Hub — confirm first.
    if (cacheDir !== originalCacheDir && !confirmingMove) {
      confirmingMove = true;
      return;
    }
    void save();
  }

  async function save() {
    saving = true;
    error = "";
    try {
      const backend = data.backends.find((b) => b.profile === profile);
      await api.lumenSave({ preset, backend: backend?.name ?? "", profile, cacheDir });
      open = false;
      setTimeout(() => void refreshState(), 350);
    } catch (e) {
      error = String(e);
    } finally {
      saving = false;
    }
  }
</script>

<Dialog.Root bind:open>
  <Dialog.Portal>
    <Dialog.Overlay class="fixed inset-0 z-40 bg-black/45" />
    <Dialog.Content
      class="fixed top-1/2 left-1/2 z-50 flex max-h-[85vh] w-[560px] max-w-[90vw] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-[10px] border border-line bg-raised shadow-xl"
    >
      <div class="flex items-center justify-between border-b border-line px-4 py-3">
        <Dialog.Title class="text-sm font-semibold">{t("configureTitle")}</Dialog.Title>
        <Dialog.Close class="btn btn-ghost btn-sm">{t("close")}</Dialog.Close>
      </div>

      <div class="flex flex-1 flex-col gap-3.5 overflow-auto px-4 py-3.5">
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
          <p class="m-0 text-xs text-warning">{t("cacheMoveWarning")}</p>
        {/if}
        {#if error}
          <p class="m-0 text-xs text-error">{error}</p>
        {/if}
      </div>

      <div class="flex justify-end gap-2 border-t border-line px-4 py-3">
        <button class="btn btn-ghost btn-sm" onclick={() => (open = false)}>{t("cancel")}</button>
        <button class="btn btn-primary btn-sm" disabled={saving} onclick={requestSave}>
          {confirmingMove ? t("confirmMove") : t("save")}
        </button>
      </div>
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>
