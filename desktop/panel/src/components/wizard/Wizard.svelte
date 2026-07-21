<script lang="ts">
  import { Checkbox } from "bits-ui";
  import { fly } from "svelte/transition";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { i18n } from "../../lib/i18n.svelte.ts";
  import { enterDashboard, refreshState, store } from "../../lib/store.svelte.ts";
  import type { Validation } from "../../lib/types.ts";
  import BackendPicker from "../shared/BackendPicker.svelte";
  import LegalDialog, { type LegalDoc } from "../shared/LegalDialog.svelte";
  import PathPicker from "../shared/PathPicker.svelte";
  import PresetPicker from "../shared/PresetPicker.svelte";
  import RegionSelect from "../shared/RegionSelect.svelte";

  const data = $derived(store.data!);

  let step = $state(1);
  let direction = $state(1);

  let path = $state("");
  let validation = $state<Validation | null>(null);
  let region = $state("other");
  let agreed = $state(false);
  let legalDoc = $state<LegalDoc | null>(null);

  let profile = $state("");
  let preset = $state("");
  let cacheDir = $state("");
  let cacheValidation = $state<Validation | null>(null);

  // Skipping step 2 or 3 records local AI as off-by-choice; pressing Continue
  // on the final step re-enables it.
  let enableLumen = $state(true);

  let submitting = $state(false);

  let seeded = false;
  $effect.pre(() => {
    // Seed wizard fields once from the backend state; later refreshes must not
    // overwrite in-progress user choices.
    if (seeded) return;
    seeded = true;
    path = data.path;
    validation = data.validation;
    if (data.region === "cn") region = "cn";
    // Recommended options arrive pre-selected, not merely badged.
    const rec = data.backends.find((b) => b.recommended) ?? data.backends[0];
    profile = data.lumen.profile || rec?.profile || "";
    preset = data.lumen.preset || data.recommendedPreset;
    cacheDir = data.lumen.cacheDir;
    cacheValidation = data.cacheValidation;
  });

  const reducedMotion =
    typeof matchMedia !== "undefined" && matchMedia("(prefers-reduced-motion: reduce)").matches;
  const slide = $derived({ x: 24 * direction, duration: reducedMotion ? 0 : 150 });
  const slideOut = $derived({ x: -24 * direction, duration: reducedMotion ? 0 : 150 });

  const canContinue = $derived(
    step === 1 ? agreed && (validation?.writable ?? false) : !submitting,
  );

  async function pickCache() {
    const r = await api.pickCache();
    if (!r.cancelled && r.path) {
      cacheDir = r.path;
      cacheValidation = r.validation ?? null;
    }
  }

  function goTo(next: number) {
    direction = next > step ? 1 : -1;
    step = next;
  }

  function skip() {
    enableLumen = false;
    if (step >= 3) void finish();
    else goTo(step + 1);
  }

  function next() {
    if (!canContinue) return;
    if (step >= 3) {
      enableLumen = true;
      void finish();
    } else {
      goTo(step + 1);
    }
  }

  async function finish() {
    if (submitting) return;
    submitting = true;
    try {
      const backend = data.backends.find((b) => b.profile === profile);
      await api.complete({
        path,
        lang: i18n.lang,
        region,
        agreed,
        enableLumen,
        preset,
        backend: backend?.name ?? "",
        profile,
        cacheDir,
      });
      enterDashboard();
      void refreshState();
      setTimeout(() => void refreshState(), 700);
    } catch (e) {
      store.error = String(e);
    } finally {
      submitting = false;
    }
  }
</script>

<div class="flex min-h-0 flex-1 flex-col px-7 pt-5">
  <div class="relative min-h-0 flex-1 overflow-hidden">
    {#key step}
      <div
        class="absolute inset-0 flex flex-col gap-3.5 overflow-auto pb-4"
        in:fly={slide}
        out:fly={slideOut}
      >
        <div class="text-[11px] font-semibold tracking-widest text-muted uppercase">
          {t("stepOf", { n: step })}
        </div>

        {#if step === 1}
          <h1 class="m-0 text-xl font-semibold">{t("s1title")}</h1>
          <p class="m-0 text-[13px] leading-normal text-muted">{t("s1sub")}</p>

          <PathPicker label={t("pathLabel")} {path} {validation} readonly />

          <div class="flex flex-col gap-2 rounded-[10px] border border-line bg-raised px-4 py-3.5">
            <div class="text-xs font-semibold tracking-wide text-muted uppercase">
              {t("regionLabel")}
            </div>
            <RegionSelect bind:region />
          </div>

          <div class="flex flex-col gap-2 rounded-[10px] border border-line bg-raised px-4 py-3.5">
            <label class="flex cursor-pointer items-start gap-2.5">
              <Checkbox.Root
                bind:checked={agreed}
                class="mt-0.5 flex h-[15px] w-[15px] shrink-0 items-center justify-center rounded-[4px] border border-line bg-surface focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none data-[state=checked]:border-primary data-[state=checked]:bg-primary"
              >
                {#snippet children({ checked })}
                  {#if checked}
                    <svg viewBox="0 0 12 12" class="h-2.5 w-2.5 text-primary-content" aria-hidden="true">
                      <path
                        d="M2.5 6.5 5 9l4.5-5.5"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="1.8"
                        stroke-linecap="round"
                        stroke-linejoin="round"
                      />
                    </svg>
                  {/if}
                {/snippet}
              </Checkbox.Root>
              <span class="text-[13px] leading-snug">{t("agree")}</span>
            </label>
            <div class="flex flex-wrap items-center gap-2 pl-6">
              <button class="link text-xs text-primary" onclick={() => (legalDoc = "terms")}>
                {t("termsLink")}
              </button>
              <span class="text-xs text-muted">·</span>
              <button class="link text-xs text-primary" onclick={() => (legalDoc = "license")}>
                {t("gplLink")}
              </button>
              <span class="text-xs text-muted">·</span>
              <button class="link text-xs text-primary" onclick={() => (legalDoc = "third-party")}>
                {t("thirdPartyLink")}
              </button>
            </div>
          </div>
        {:else if step === 2}
          <h1 class="m-0 text-xl font-semibold">{t("s2title")}</h1>
          <p class="m-0 text-[13px] leading-normal text-muted">{t("s2sub")}</p>
          <BackendPicker backends={data.backends} bind:profile />
        {:else}
          <h1 class="m-0 text-xl font-semibold">{t("s3title")}</h1>
          <p class="m-0 text-[13px] leading-normal text-muted">{t("s3sub")}</p>
          <PresetPicker presets={data.presets} recommended={data.recommendedPreset} bind:preset />
          <p class="m-0 text-xs text-muted">{t("modelsDownloadNote")}</p>
          <PathPicker
            label={t("cacheLabel")}
            path={cacheDir}
            validation={cacheValidation}
            onpick={pickCache}
          />
        {/if}
      </div>
    {/key}
  </div>

  <div class="-mx-7 flex shrink-0 items-center gap-3.5 border-t border-line px-7 py-3.5">
    {#if step > 1}
      <button class="btn btn-ghost btn-sm text-muted" onclick={() => goTo(step - 1)}>
        {t("back")}
      </button>
    {:else}
      <span></span>
    {/if}
    <div class="mx-auto flex gap-1.5">
      {#each [1, 2, 3] as n (n)}
        <span class={`h-1.5 w-1.5 rounded-full ${n === step ? "bg-primary" : "bg-line"}`}></span>
      {/each}
    </div>
    <div class="flex items-center gap-2.5">
      {#if step > 1}
        <button class="btn btn-ghost btn-sm text-muted" onclick={skip}>{t("setUpLater")}</button>
      {/if}
      <button class="btn btn-primary btn-sm" disabled={!canContinue} onclick={next}>
        {step === 3 ? t("finish") : t("continueLbl")}
      </button>
    </div>
  </div>
</div>

<LegalDialog bind:doc={legalDoc} />
