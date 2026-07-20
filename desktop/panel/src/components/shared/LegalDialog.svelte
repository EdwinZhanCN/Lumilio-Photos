<script module lang="ts">
  export type LegalDoc = "terms" | "license" | "third-party";
</script>

<script lang="ts">
  import { Dialog } from "bits-ui";
  import { api } from "../../lib/api.ts";
  import { i18n, t } from "../../lib/i18n.svelte.ts";

  let {
    doc = $bindable(),
  }: {
    doc: LegalDoc | null;
  } = $props();

  let content = $state("");

  const title = $derived(
    doc === "terms" ? t("termsLink") : doc === "license" ? t("gplLink") : t("thirdPartyLink"),
  );

  $effect(() => {
    if (!doc) return;
    content = "…";
    const requested = doc;
    api
      .legal(requested, i18n.lang)
      .then((text) => {
        if (doc === requested) content = text;
      })
      .catch((e: unknown) => {
        if (doc === requested) content = String(e);
      });
  });
</script>

<Dialog.Root open={doc !== null} onOpenChange={(open) => (doc = open ? doc : null)}>
  <Dialog.Portal>
    <Dialog.Overlay class="fixed inset-0 z-40 bg-black/45" />
    <Dialog.Content
      class="fixed top-1/2 left-1/2 z-50 flex h-[480px] max-h-[85vh] w-[560px] max-w-[90vw] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-[10px] border border-line bg-raised shadow-xl"
    >
      <div class="flex items-center justify-between border-b border-line px-4 py-3">
        <Dialog.Title class="text-sm font-semibold">{title}</Dialog.Title>
        <Dialog.Close class="btn btn-ghost btn-sm">{t("close")}</Dialog.Close>
      </div>
      <div class="flex-1 overflow-auto px-4 py-3.5">
        <pre class="m-0 font-mono text-xs leading-relaxed whitespace-pre-wrap">{content}</pre>
      </div>
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>
