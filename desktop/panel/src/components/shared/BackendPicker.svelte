<script lang="ts">
  import { RadioGroup } from "bits-ui";
  import { t } from "../../lib/i18n.svelte.ts";
  import type { BackendChoice } from "../../lib/types.ts";

  let {
    backends,
    profile = $bindable(),
  }: {
    backends: BackendChoice[];
    profile: string;
  } = $props();

  function title(b: BackendChoice): string {
    switch (b.name) {
      case "metal":
        return t("backendMetal");
      case "gpu":
        return t("backendGpu");
      case "cpu":
        return t("backendCpu");
      default:
        return b.name.toUpperCase();
    }
  }

  function desc(b: BackendChoice): string {
    switch (b.name) {
      case "metal":
        return t("backendMetalDesc");
      case "gpu":
        return t("backendGpuDesc");
      case "cpu":
        return t("backendCpuDesc");
      default:
        return b.profile;
    }
  }
</script>

<RadioGroup.Root bind:value={profile} class="grid grid-cols-2 gap-2.5">
  {#each backends as b (b.profile)}
    <RadioGroup.Item
      value={b.profile}
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
        <span class="flex-1 text-[13.5px] font-semibold">{title(b)}</span>
        {#if b.recommended}
          <span
            class="rounded-[5px] bg-accent-soft px-1.5 py-0.5 text-[10px] font-bold text-primary-content"
          >
            {t("recommended")}
          </span>
        {/if}
      </div>
      <p class="m-0 text-xs leading-snug text-muted">{desc(b)}</p>
    </RadioGroup.Item>
  {/each}
</RadioGroup.Root>
