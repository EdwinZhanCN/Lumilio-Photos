<script lang="ts">
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import {
    acceptRuntimeValidation,
    resetRuntimeDraft,
    runtimeDraft,
    runtimeDraftDirty,
  } from "../../lib/runtime-draft.svelte.ts";
  import { refreshState, store } from "../../lib/store.svelte.ts";

  let {
    dirty = $bindable(false),
    saving = $bindable(false),
  }: {
    dirty?: boolean;
    saving?: boolean;
  } = $props();
  let notice = $state("");
  let validating = $state(false);

  $effect(() => {
    dirty = runtimeDraftDirty();
  });

  const currentValidation = $derived(
    runtimeDraft.candidateToml === runtimeDraft.validatedCandidate
      ? runtimeDraft.valid
      : null,
  );

  export async function save(): Promise<boolean> {
    if (!runtimeDraft.view) return false;
    saving = true;
    runtimeDraft.error = "";
    notice = "";
    try {
      const validation = await api.validateRuntimeConfig({
        baseFingerprint: runtimeDraft.view.baseFingerprint,
        toml: runtimeDraft.candidateToml,
      });
      acceptRuntimeValidation(validation);
      if (!validation.valid) return false;
      await api.applyRuntimeConfig({
        baseFingerprint: runtimeDraft.view.baseFingerprint,
        toml: validation.candidateToml,
      });
      setTimeout(() => void refreshState(), 250);
      return true;
    } catch (cause) {
      runtimeDraft.error = cause instanceof Error ? cause.message : String(cause);
      return false;
    } finally {
      saving = false;
    }
  }

  async function validateCandidate(): Promise<void> {
    if (!runtimeDraft.view) return;
    validating = true;
    runtimeDraft.error = "";
    notice = "";
    try {
      const validation = await api.validateRuntimeConfig({
        baseFingerprint: runtimeDraft.view.baseFingerprint,
        toml: runtimeDraft.candidateToml,
      });
      acceptRuntimeValidation(validation);
    } catch (cause) {
      runtimeDraft.error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      validating = false;
    }
  }

  async function restoreLastKnownGood(): Promise<void> {
    saving = true;
    runtimeDraft.error = "";
    notice = "";
    try {
      await api.restoreRuntimeConfig();
      notice = t("restoreAccepted");
      setTimeout(() => void refreshState(), 250);
    } catch (cause) {
      runtimeDraft.error = cause instanceof Error ? cause.message : String(cause);
    } finally {
      saving = false;
    }
  }
</script>

<div class="flex flex-col gap-3.5">
  <div>
    <h3 class="m-0 text-sm font-semibold">{t("runtimeConfiguration")}</h3>
    <p class="m-0 mt-1 text-xs text-muted">{t("runtimeConfigurationHint")}</p>
  </div>

  {#if runtimeDraft.loading}
    <div class="flex items-center gap-2 text-xs text-muted">
      <span class="loading loading-bars loading-sm text-primary"></span>
      <span>{t("loadingRuntimeConfiguration")}</span>
    </div>
  {:else if runtimeDraft.view}
    <details class="collapse collapse-arrow border border-line bg-surface">
      <summary class="collapse-title min-h-0 px-3 py-2.5 text-xs font-semibold">
        {t("currentRuntime")}
      </summary>
      <div class="collapse-content px-3 pb-3">
        <pre
          class="m-0 max-h-44 overflow-auto rounded-md bg-base-300 p-2.5 font-mono text-[11px] leading-relaxed whitespace-pre"
        >{runtimeDraft.view.currentToml}</pre>
      </div>
    </details>

    <label class="flex min-w-0 flex-col gap-1.5 text-xs font-semibold">
      <span>{t("candidateConfiguration")}</span>
      <textarea
        class="textarea min-h-64 w-full resize-y font-mono text-[11px] leading-relaxed"
        spellcheck="false"
        bind:value={runtimeDraft.candidateToml}
        aria-describedby="runtime-candidate-help"
      ></textarea>
    </label>
    <p id="runtime-candidate-help" class="m-0 text-xs text-muted">
      {t("candidateConfigurationHint")}
    </p>

    <div class="flex flex-wrap gap-2">
      <button
        class="btn btn-secondary btn-sm"
        disabled={saving || validating}
        onclick={() => void validateCandidate()}
      >
        {validating ? t("validatingCandidate") : t("validateCandidate")}
      </button>
      <button class="btn btn-ghost btn-sm" disabled={!dirty} onclick={resetRuntimeDraft}>
        {t("resetCandidate")}
      </button>
      <button
        class="btn btn-ghost btn-sm"
        disabled={!store.data?.paths.serverConfig}
        onclick={() => void api.openPath(store.data!.paths.serverConfig!)}
      >
        {t("revealActiveManifest")}
      </button>
      <button
        class="btn btn-ghost btn-sm"
        disabled={saving || !runtimeDraft.view.lastKnownGoodAvailable}
        onclick={() => void restoreLastKnownGood()}
      >
        {t("restoreLastKnownGood")}
      </button>
    </div>

    {#if currentValidation === true}
      <div role="status" class="alert alert-success alert-soft text-xs">
        <span>
          {runtimeDraft.semanticChanges.length > 0
            ? t("candidateValidRestart")
            : t("candidateValidNoChanges")}
        </span>
      </div>
    {:else if currentValidation === false ||
    (runtimeDraft.candidateToml === runtimeDraft.validatedCandidate &&
      runtimeDraft.issues.length > 0)}
      <div role="alert" class="alert alert-error alert-soft items-start text-xs">
        <ul class="m-0 list-disc space-y-1 pl-4">
          {#each runtimeDraft.issues as issue (`${issue.field}-${issue.code}-${issue.message}`)}
            <li>
              {#if issue.field}<code>{issue.field}</code>: {/if}{issue.message}
            </li>
          {/each}
        </ul>
      </div>
    {/if}

    {#if runtimeDraft.semanticChanges.length > 0 &&
    runtimeDraft.candidateToml === runtimeDraft.validatedCandidate}
      <div class="rounded-md border border-line bg-surface p-3">
        <div class="mb-2 text-xs font-semibold">{t("semanticChanges")}</div>
        <dl class="m-0 grid gap-2 text-xs">
          {#each runtimeDraft.semanticChanges as change (change.field)}
            <div>
              <dt class="font-mono text-[11px] text-muted">{change.field}</dt>
              <dd class="m-0 mt-0.5 break-words">{change.before} → {change.after}</dd>
            </div>
          {/each}
        </dl>
      </div>
    {/if}

    <details class="text-xs text-muted">
      <summary class="cursor-pointer font-semibold">{t("hostManagedFields")}</summary>
      <p class="mb-1">{t("hostManagedFieldsHint")}</p>
      <code class="block break-words">{runtimeDraft.view.hostManagedPaths.join(", ")}</code>
    </details>
  {/if}

  {#if runtimeDraft.error}
    <div role="alert" class="alert alert-error alert-soft text-xs">
      <span>{runtimeDraft.error}</span>
    </div>
  {:else if notice}
    <div role="status" class="alert alert-success alert-soft text-xs">
      <span>{notice}</span>
    </div>
  {/if}
</div>
