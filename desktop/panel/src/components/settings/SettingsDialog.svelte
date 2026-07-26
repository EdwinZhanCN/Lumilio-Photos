<script module lang="ts">
  export type SettingsTab = "general" | "server" | "lumen" | "runtime";
</script>

<script lang="ts">
  import { Dialog } from "bits-ui";
  import { t } from "../../lib/i18n.svelte.ts";
  import { loadRuntimeDraft } from "../../lib/runtime-draft.svelte.ts";
  import GeneralSettingsForm from "./GeneralSettingsForm.svelte";
  import LumenSettingsForm from "./LumenSettingsForm.svelte";
  import RuntimeSettingsForm from "./RuntimeSettingsForm.svelte";
  import ServerSettingsForm from "./ServerSettingsForm.svelte";

  let {
    open = $bindable(false),
    tab = $bindable<SettingsTab>("general"),
    session,
  }: {
    open?: boolean;
    tab?: SettingsTab;
    session: number;
  } = $props();

  let generalForm: GeneralSettingsForm | undefined = undefined;
  let serverForm: ServerSettingsForm | undefined = undefined;
  let lumenForm: LumenSettingsForm | undefined = undefined;
  let runtimeForm: RuntimeSettingsForm | undefined = undefined;
  let generalDirty = $state(false);
  let generalSaving = $state(false);
  let serverDirty = $state(false);
  let serverSaving = $state(false);
  let serverCanSave = $state(true);
  let lumenDirty = $state(false);
  let lumenSaving = $state(false);
  let lumenConfirmingMove = $state(false);
  let runtimeDirty = $state(false);
  let runtimeSaving = $state(false);

  const tabs: Array<{ id: SettingsTab; label: () => string }> = [
    { id: "general", label: () => t("settingsTabGeneral") },
    { id: "server", label: () => t("settingsTabServer") },
    { id: "lumen", label: () => t("settingsTabLumen") },
    { id: "runtime", label: () => t("settingsTabRuntime") },
  ];

  const anyDirty = $derived(generalDirty || serverDirty || lumenDirty || runtimeDirty);
  const activeDirty = $derived(
    tab === "general"
      ? generalDirty
      : tab === "server"
        ? serverDirty
        : tab === "lumen"
          ? lumenDirty
          : runtimeDirty,
  );
  const activeSaving = $derived(
    tab === "general"
      ? generalSaving
      : tab === "server"
        ? serverSaving
        : tab === "lumen"
          ? lumenSaving
          : runtimeSaving,
  );
  const activeCanSave = $derived(tab !== "server" || serverCanSave);
  const saveLabel = $derived(
    tab === "runtime"
      ? t("validateApplyRestart")
      : tab === "server"
      ? t("networkSave")
      : tab === "lumen" && lumenConfirmingMove
        ? t("confirmMove")
        : t("saveChanges"),
  );

  $effect(() => {
    if (!open) return;
    void session;
    void loadRuntimeDraft(session);
  });

  function requestClose(nextOpen: boolean): void {
    if (nextOpen) {
      open = true;
      return;
    }
    if (anyDirty && !window.confirm(t("discardSettingsConfirm"))) return;
    open = false;
  }

  async function saveActive(): Promise<void> {
    if (!generalForm || !serverForm || !lumenForm || !runtimeForm) return;
    const saved =
      tab === "general"
        ? await generalForm.save()
        : tab === "server"
          ? await serverForm.save()
          : tab === "lumen"
            ? await lumenForm.save()
            : await runtimeForm.save();
    if (saved) open = false;
  }
</script>

<Dialog.Root {open} onOpenChange={requestClose}>
  <Dialog.Portal>
    <Dialog.Overlay class="fixed inset-0 z-40 bg-black/45" />
    <Dialog.Content
      class="fixed top-1/2 left-1/2 z-50 flex max-h-[88vh] w-[720px] max-w-[94vw] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-[10px] border border-line bg-raised shadow-xl"
    >
      <header class="flex items-center justify-between gap-3 border-b border-line px-4 py-3">
        <div>
          <Dialog.Title class="text-sm font-semibold">{t("settingsTitle")}</Dialog.Title>
          <Dialog.Description class="mt-0.5 text-xs text-muted">
            {t("settingsDescription")}
          </Dialog.Description>
        </div>
        <button class="btn btn-ghost btn-sm" aria-label={t("close")} onclick={() => requestClose(false)}>
          {t("close")}
        </button>
      </header>

      <div class="grid min-h-0 flex-1 grid-cols-1 min-[640px]:grid-cols-[150px_minmax(0,1fr)]">
        <div
          class="flex gap-1 overflow-x-auto border-b border-line p-2 min-[640px]:flex-col min-[640px]:border-r min-[640px]:border-b-0"
          role="tablist"
          aria-label={t("settingsSections")}
        >
          {#each tabs as item (item.id)}
            <button
              type="button"
              role="tab"
              id={`settings-tab-${item.id}`}
              aria-selected={tab === item.id}
              aria-controls={`settings-panel-${item.id}`}
              class="btn btn-ghost btn-sm justify-start whitespace-nowrap data-[active=true]:bg-accent-soft data-[active=true]:text-base-content"
              data-active={tab === item.id}
              onclick={() => (tab = item.id)}
            >
              {item.label()}
              {#if (item.id === "general" && generalDirty) ||
              (item.id === "server" && serverDirty) ||
              (item.id === "lumen" && lumenDirty) ||
              (item.id === "runtime" && runtimeDirty)}
                <span class="status status-warning status-sm" aria-label={t("unsavedChanges")}></span>
              {/if}
            </button>
          {/each}
        </div>

        <div class="min-h-0 overflow-auto px-4 py-4">
          <div
            id="settings-panel-general"
            role="tabpanel"
            aria-labelledby="settings-tab-general"
            hidden={tab !== "general"}
          >
            <GeneralSettingsForm
              bind:this={generalForm}
              {open}
              {session}
              bind:dirty={generalDirty}
              bind:saving={generalSaving}
            />
          </div>
          <div
            id="settings-panel-server"
            role="tabpanel"
            aria-labelledby="settings-tab-server"
            hidden={tab !== "server"}
          >
            <ServerSettingsForm
              bind:this={serverForm}
              {open}
              {session}
              bind:dirty={serverDirty}
              bind:saving={serverSaving}
              bind:canSave={serverCanSave}
            />
          </div>
          <div
            id="settings-panel-lumen"
            role="tabpanel"
            aria-labelledby="settings-tab-lumen"
            hidden={tab !== "lumen"}
          >
            <LumenSettingsForm
              bind:this={lumenForm}
              {open}
              {session}
              bind:dirty={lumenDirty}
              bind:saving={lumenSaving}
              bind:confirmingMove={lumenConfirmingMove}
            />
          </div>
          <div
            id="settings-panel-runtime"
            role="tabpanel"
            aria-labelledby="settings-tab-runtime"
            hidden={tab !== "runtime"}
          >
            <RuntimeSettingsForm
              bind:this={runtimeForm}
              bind:dirty={runtimeDirty}
              bind:saving={runtimeSaving}
            />
          </div>
        </div>
      </div>

      <footer class="flex justify-end gap-2 border-t border-line px-4 py-3">
        <button class="btn btn-ghost btn-sm" disabled={activeSaving} onclick={() => requestClose(false)}>
          {t("cancel")}
        </button>
        <button
          class="btn btn-primary btn-sm"
          disabled={activeSaving || !activeDirty || !activeCanSave}
          onclick={() => void saveActive()}
        >
          {activeSaving ? t("saving") : saveLabel}
        </button>
      </footer>
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>
