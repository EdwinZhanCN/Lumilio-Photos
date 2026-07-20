<script lang="ts">
  import { RadioGroup } from "bits-ui";
  import { t } from "../../lib/i18n.svelte.ts";
  import type { Preset } from "../../lib/types.ts";

  let {
    presets,
    recommended,
    preset = $bindable(),
  }: {
    presets: Preset[];
    recommended: string;
    preset: string;
  } = $props();

  function title(p: Preset): string {
    switch (p.name) {
      case "minimal":
        return t("minimal");
      case "basic":
        return t("basic");
      case "brave":
        return t("brave");
      default:
        return p.name;
    }
  }

  function desc(p: Preset): string {
    switch (p.name) {
      case "minimal":
        return t("minimalDesc");
      case "basic":
        return t("basicDesc");
      case "brave":
        return t("braveDesc");
      default:
        return "";
    }
  }
</script>

<RadioGroup.Root bind:value={preset} class="grid grid-cols-3 gap-2.5">
  {#each presets as p (p.name)}
    <RadioGroup.Item
      value={p.name}
      class="group flex cursor-pointer flex-col gap-1.5 rounded-[10px] border border-line bg-raised p-3 text-left transition-colors focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none data-[state=checked]:border-primary data-[state=checked]:bg-accent-soft"
    >
      <div class="flex items-center gap-2">
        <span
          class="flex h-[15px] w-[15px] shrink-0 items-center justify-center rounded-full border-[1.5px] border-line group-data-[state=checked]:border-primary"
        >
          <span
            class="h-[7px] w-[7px] rounded-full bg-transparent group-data-[state=checked]:bg-primary"
          ></span>
        </span>
        <span class="flex-1 text-[13.5px] font-semibold">{title(p)}</span>
        {#if p.name === recommended}
          <span
            class="rounded-[5px] bg-accent-soft px-1.5 py-0.5 text-[10px] font-bold text-primary-content"
          >
            {t("recommended")}
          </span>
        {/if}
      </div>
      <p class="m-0 text-xs leading-snug text-muted">{desc(p)}</p>
      <p class="m-0 font-mono text-[11px] text-muted">
        {t("presetSpec", { ram: p.minRamGB, disk: p.minDiskGB })}
      </p>
    </RadioGroup.Item>
  {/each}
</RadioGroup.Root>
