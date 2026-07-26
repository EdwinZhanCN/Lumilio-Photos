<script lang="ts">
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { store } from "../../lib/store.svelte.ts";
  import type {
    ConfigIssue,
    RuntimeConfigValidation,
    RuntimeConfigView,
    SemanticChange,
  } from "../../lib/types.ts";

  let {
    open,
    session,
    dirty = $bindable(false),
    saving = $bindable(false),
  }: {
    open: boolean;
    session: number;
    dirty?: boolean;
    saving?: boolean;
  } = $props();

  let view = $state<RuntimeConfigView | null>(null);
  let candidate = $state("");
  let issues = $state<ConfigIssue[]>([]);
  let changes = $state<SemanticChange[]>([]);
  let valid = $state<boolean | null>(null);
  let validatedCandidate = $state("");
  let loading = $state(false);
  let loadError = $state("");
  let loadToken = 0;

  $effect(() => {
    if (!open) return;
    void session;
    const token = ++loadToken;
    loading = true;
    loadError = "";
    api
      .runtimeConfig()
      .then((result) => {
        if (token !== loadToken) return;
        view = result;
        candidate = result.candidateToml;
        issues = result.issues;
        changes = result.semanticChanges;
        valid = null;
        validatedCandidate = result.issues.length > 0 ? result.candidateToml : "";
      })
      .catch((cause: unknown) => {
        if (token === loadToken) {
          loadError = cause instanceof Error ? cause.message : String(cause);
        }
      })
      .finally(() => {
        if (token === loadToken) loading = false;
      });
  });

  $effect(() => {
    dirty = Boolean(view && candidate !== view.currentToml);
  });

  const currentValidation = $derived(candidate === validatedCandidate ? valid : null);

  function acceptValidation(result: RuntimeConfigValidation): void {
    issues = result.issues;
    changes = result.semanticChanges;
    valid = result.valid;
    if (result.candidateToml) candidate = result.candidateToml;
    validatedCandidate = candidate;
  }

  export async function save(): Promise<boolean> {
    if (!view) return false;
    saving = true;
    loadError = "";
    try {
      acceptValidation(
        await api.validateRuntimeConfig({
          baseFingerprint: view.baseFingerprint,
          toml: candidate,
        }),
      );
    } catch (cause) {
      loadError = cause instanceof Error ? cause.message : String(cause);
    } finally {
      saving = false;
    }
    return false;
  }

  function resetCandidate(): void {
    if (!view) return;
    candidate = view.currentToml;
    issues = [];
    changes = [];
    valid = null;
    validatedCandidate = "";
  }
</script>

<div class="flex flex-col gap-3.5">
  <div>
    <h3 class="m-0 text-sm font-semibold">{t("runtimeConfiguration")}</h3>
    <p class="m-0 mt-1 text-xs text-muted">{t("runtimeConfigurationHint")}</p>
  </div>

  {#if loading}
    <div class="flex items-center gap-2 text-xs text-muted">
      <span class="loading loading-bars loading-sm text-primary"></span>
      <span>{t("loadingRuntimeConfiguration")}</span>
    </div>
  {:else if view}
    <details class="collapse collapse-arrow border border-line bg-surface">
      <summary class="collapse-title min-h-0 px-3 py-2.5 text-xs font-semibold">
        {t("currentRuntime")}
      </summary>
      <div class="collapse-content px-3 pb-3">
        <pre
          class="m-0 max-h-44 overflow-auto rounded-md bg-base-300 p-2.5 font-mono text-[11px] leading-relaxed whitespace-pre"
        >{view.currentToml}</pre>
      </div>
    </details>

    <label class="flex min-w-0 flex-col gap-1.5 text-xs font-semibold">
      <span>{t("candidateConfiguration")}</span>
      <textarea
        class="textarea min-h-64 w-full resize-y font-mono text-[11px] leading-relaxed"
        spellcheck="false"
        bind:value={candidate}
        aria-describedby="runtime-candidate-help"
      ></textarea>
    </label>
    <p id="runtime-candidate-help" class="m-0 text-xs text-muted">
      {t("candidateConfigurationHint")}
    </p>

    <div class="flex flex-wrap gap-2">
      <button class="btn btn-ghost btn-sm" disabled={!dirty} onclick={resetCandidate}>
        {t("resetCandidate")}
      </button>
      <button
        class="btn btn-ghost btn-sm"
        disabled={!store.data?.paths.serverConfig}
        onclick={() => void api.openPath(store.data!.paths.serverConfig!)}
      >
        {t("revealActiveManifest")}
      </button>
    </div>

    {#if currentValidation === true}
      <div role="status" class="alert alert-success alert-soft text-xs">
        <span>{changes.length > 0 ? t("candidateValidRestart") : t("candidateValidNoChanges")}</span>
      </div>
    {:else if currentValidation === false ||
    (candidate === validatedCandidate && issues.length > 0)}
      <div role="alert" class="alert alert-error alert-soft items-start text-xs">
        <ul class="m-0 list-disc space-y-1 pl-4">
          {#each issues as issue (`${issue.field}-${issue.code}-${issue.message}`)}
            <li>
              {#if issue.field}<code>{issue.field}</code>: {/if}{issue.message}
            </li>
          {/each}
        </ul>
      </div>
    {/if}

    {#if changes.length > 0 && candidate === validatedCandidate}
      <div class="rounded-md border border-line bg-surface p-3">
        <div class="mb-2 text-xs font-semibold">{t("semanticChanges")}</div>
        <dl class="m-0 grid gap-2 text-xs">
          {#each changes as change (change.field)}
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
      <code class="block break-words">{view.hostManagedPaths.join(", ")}</code>
    </details>
  {/if}

  {#if loadError}
    <div role="alert" class="alert alert-error alert-soft text-xs"><span>{loadError}</span></div>
  {/if}
</div>
