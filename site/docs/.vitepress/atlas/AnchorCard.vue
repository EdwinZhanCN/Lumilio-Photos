<script setup lang="ts">
// One anchor: what an element is bound to in the code, and a door to open it.
import { computed } from 'vue'
import { Code2, Database, Globe, Boxes, Layers, FileText, ExternalLink, AlertTriangle } from '@lucide/vue'
import { type AtlasData, sourceUrl } from './model'

const props = defineProps<{ data: AtlasData; anchor?: string; openIn: 'vscode' | 'github' }>()
const emit = defineEmits<{ module: [id: string]; group: [id: string] }>()

const resolved = computed(() => (props.anchor ? props.data.anchors[props.anchor] : undefined))
const kind = computed(() => props.anchor?.split(':', 1)[0] ?? '')
const reference = computed(() => props.anchor?.slice(kind.value.length + 1) ?? '')
const icon = computed(
  () =>
    ({ go: Code2, ts: Code2, api: Globe, sql: Database, mod: Boxes, group: Layers, file: FileText, ext: ExternalLink })[
      kind.value
    ] ?? Code2,
)
const href = computed(() =>
  resolved.value?.file ? sourceUrl(props.data, resolved.value.file, resolved.value.line, props.openIn) : undefined,
)
const location = computed(() =>
  resolved.value?.file ? `${resolved.value.file.split('/').slice(-2).join('/')}${resolved.value.line ? `:${resolved.value.line}` : ''}` : '',
)
</script>

<template>
  <div v-if="anchor" class="anchor-card" :class="{ stale: resolved?.stale }">
    <component :is="icon" class="anchor-icon" :size="15" />
    <div class="anchor-body">
      <div class="anchor-ref">
        <span class="anchor-kind">{{ kind }}</span>
        <code>{{ reference }}</code>
      </div>
      <div class="anchor-meta">
        <a v-if="href" :href="href" class="anchor-open" :title="resolved?.file">{{ location }}</a>
        <button v-if="resolved?.module && kind !== 'mod'" class="anchor-link" @click="emit('module', resolved.module!)">module</button>
        <button v-if="kind === 'mod' && resolved?.module" class="anchor-link" @click="emit('module', resolved.module!)">open module</button>
        <button v-if="kind === 'group'" class="anchor-link" @click="emit('group', reference)">enter group</button>
        <span v-if="kind === 'ext'" class="anchor-muted">outside this repository</span>
        <span v-if="resolved?.stale" class="anchor-stale"><AlertTriangle :size="12" /> re-verify</span>
      </div>
    </div>
  </div>
</template>
