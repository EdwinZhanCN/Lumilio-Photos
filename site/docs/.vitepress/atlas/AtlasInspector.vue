<script setup lang="ts">
// The inspector is where text lives. The canvas shows names; this panel
// answers "what is this, where is it in the code, and where can I go next".
import { computed, ref, watch } from 'vue'
import { ArrowLeft, ArrowRight, ArrowUpRight, ChevronRight, X, CornerDownRight } from '@lucide/vue'
import AnchorCard from './AnchorCard.vue'
import {
  type AtlasData,
  type AtlasModule,
  type AtlasSymbol,
  type AtlasView,
  KIND_LABEL,
  SIDE_LABEL,
  shortModule,
  sourceUrl,
} from './model'

const props = defineProps<{
  data: AtlasData
  view?: AtlasView
  module?: AtlasModule
  selected?: string | null
  openIn: 'vscode' | 'github'
  symbols: AtlasSymbol[] | null
}>()

const emit = defineEmits<{
  close: []
  select: [id: string | null]
  view: [id: string, select?: string]
  module: [id: string]
  step: [delta: number]
  loadSymbols: []
}>()

const modules = computed(() => new Map(props.data.modules.map((module) => [module.id, module])))
const groups = computed(() => new Map(props.data.groups.map((group) => [group.id, group])))
const views = computed(() => new Map(props.data.views.map((view) => [view.id, view])))

const node = computed(() => props.view?.nodes.find((item) => item.id === props.selected))
const row = computed(() => {
  if (!props.view?.rows || !props.selected?.startsWith('step:')) return undefined
  return props.view.rows[Number(props.selected.slice(5)) - 1]
})
const label = (id: string) => props.view?.nodes.find((item) => item.id === id)?.label ?? id

const links = computed(() => props.view?.edges ?? props.view?.transitions ?? [])
const incoming = computed(() => links.value.filter((edge) => edge.to === props.selected))
const outgoing = computed(() => links.value.filter((edge) => edge.from === props.selected))

const nodeModule = computed(() => {
  const anchor = node.value?.anchor
  if (!anchor) return undefined
  const resolved = props.data.anchors[anchor]
  return resolved?.module ? modules.value.get(resolved.module) : undefined
})
const nodeGroup = computed(() =>
  node.value?.anchor?.startsWith('group:') ? groups.value.get(node.value.anchor.slice(6)) : undefined,
)
const alsoIn = computed(() => {
  const anchor = node.value?.anchor ?? row.value?.anchor
  if (!anchor) return []
  return (props.data.usage[anchor] ?? [])
    .filter((id) => id !== props.view?.id)
    .map((id) => views.value.get(id))
    .filter(Boolean) as AtlasView[]
})

// Module page: the selected neighbour, the module itself, and its symbols.
const focusModule = computed(() =>
  props.module && props.selected && props.selected !== props.module.id ? modules.value.get(props.selected) : undefined,
)
const docOpen = ref(false)
const symbolQuery = ref('')
watch(
  () => props.module?.id,
  () => {
    docOpen.value = false
    symbolQuery.value = ''
  },
)
const moduleSymbols = computed(() => {
  if (!props.module || !props.symbols) return []
  const query = symbolQuery.value.trim().toLowerCase()
  return props.symbols
    .filter((symbol) => symbol.module === props.module!.id && (!query || symbol.name.toLowerCase().includes(query)))
    .slice(0, 80)
})
const moduleViews = computed(() =>
  props.module ? ((props.data.usage[`mod:${props.module.id}`] ?? []).map((id) => views.value.get(id)).filter(Boolean) as AtlasView[]) : [],
)

const notesOpen = ref<number | null>(null)
watch(
  () => props.view?.id,
  () => (notesOpen.value = null),
)

function paragraphs(text: string) {
  return text
    .trim()
    .split(/\n\s*\n/)
    .map((block) =>
      block
        .replace(/[&<>]/g, (ch) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;' })[ch]!)
        .replace(/`([^`]+)`/g, '<code>$1</code>'),
    )
}
</script>

<template>
  <aside class="inspector">
    <button class="inspector-close" title="Close (Esc)" @click="emit('close')"><X :size="16" /></button>

    <!-- Sequence step -->
    <template v-if="view && row">
      <div class="inspector-eyebrow">Step {{ row.number }}<span v-if="row.block"> · {{ row.block }}</span></div>
      <h2 class="inspector-title">{{ row.label }}</h2>
      <div class="hop">
        <button class="chip-button" @click="emit('select', null)">{{ label(row.from) }}</button>
        <ArrowRight :size="14" class="hop-arrow" />
        <button class="chip-button" @click="emit('select', null)">{{ label(row.to) }}</button>
      </div>
      <AnchorCard :data="data" :anchor="row.anchor" :open-in="openIn" @module="emit('module', $event)" />
      <p v-if="!row.anchor" class="inspector-quiet">This step is narrative; it has no code anchor.</p>
      <div class="stepper">
        <button :disabled="row.number <= 1" @click="emit('step', -1)"><ArrowLeft :size="14" /> Previous <kbd>K</kbd></button>
        <button :disabled="row.number >= (view.rows?.length ?? 0)" @click="emit('step', 1)">Next <kbd>J</kbd> <ArrowRight :size="14" /></button>
      </div>
    </template>

    <!-- Node in a view -->
    <template v-else-if="view && node">
      <div class="inspector-eyebrow">{{ KIND_LABEL[view.kind] }} · {{ view.kind === 'lifecycle' ? 'state' : node.group ?? 'element' }}</div>
      <h2 class="inspector-title">{{ node.label }}</h2>
      <p v-if="node.note && !nodeModule?.synopsis && !nodeGroup" class="inspector-lede">{{ node.note }}</p>

      <template v-if="nodeGroup">
        <p class="inspector-lede">{{ nodeGroup.summary }}</p>
        <button class="primary-button" @click="emit('view', `group-${nodeGroup.id}`)">
          Enter {{ nodeGroup.title }} <CornerDownRight :size="14" />
        </button>
      </template>
      <template v-else-if="nodeModule && node.anchor?.startsWith('mod:')">
        <p class="inspector-lede">{{ nodeModule.synopsis || node.note }}</p>
        <div class="facts">
          <span>{{ nodeModule.files }} files</span><span>{{ nodeModule.lines.toLocaleString() }} lines</span>
          <span>{{ nodeModule.imports.length }} uses</span><span>{{ nodeModule.importedBy.length }} used by</span>
        </div>
        <button class="primary-button" @click="emit('module', nodeModule.id)">Open module <CornerDownRight :size="14" /></button>
      </template>
      <AnchorCard v-else :data="data" :anchor="node.anchor" :open-in="openIn" @module="emit('module', $event)" @group="emit('view', `group-${$event}`)" />

      <section v-if="incoming.length || outgoing.length" class="inspector-section">
        <h3>Connections</h3>
        <button v-for="(edge, index) in incoming" :key="`in-${index}`" class="link-row" @click="emit('select', edge.from)">
          <ArrowRight :size="13" class="link-dir in" />
          <span class="link-name">{{ label(edge.from) }}</span>
          <span class="link-label">{{ edge.label }}</span>
        </button>
        <button v-for="(edge, index) in outgoing" :key="`out-${index}`" class="link-row" @click="emit('select', edge.to)">
          <ArrowUpRight :size="13" class="link-dir out" />
          <span class="link-name">{{ label(edge.to) }}</span>
          <span class="link-label">{{ edge.label }}</span>
        </button>
      </section>

      <section v-if="alsoIn.length" class="inspector-section">
        <h3>Also appears in</h3>
        <button v-for="other in alsoIn" :key="other.id" class="link-row" @click="emit('view', other.id)">
          <span class="kind-dot" :class="other.kind" />
          <span class="link-name">{{ other.title }}</span>
          <ChevronRight :size="13" class="link-chevron" />
        </button>
      </section>
    </template>

    <!-- Module page -->
    <template v-else-if="module">
      <template v-if="focusModule">
        <div class="inspector-eyebrow">{{ SIDE_LABEL[focusModule.side] }} · {{ groups.get(focusModule.group)?.title }}</div>
        <h2 class="inspector-title mono">{{ shortModule(focusModule) }}</h2>
        <p class="inspector-lede">{{ focusModule.synopsis }}</p>
        <button class="primary-button" @click="emit('module', focusModule.id)">Open module <CornerDownRight :size="14" /></button>
        <button class="ghost-button" @click="emit('select', null)">Back to {{ shortModule(module) }}</button>
      </template>
      <template v-else>
        <div class="inspector-eyebrow">{{ SIDE_LABEL[module.side] }} · {{ module.kind }}</div>
        <h2 class="inspector-title mono">{{ shortModule(module) }}</h2>
        <p class="inspector-lede">{{ module.synopsis }}</p>
        <div class="facts">
          <button class="fact-link" @click="emit('view', `group-${module.group}`)">{{ groups.get(module.group)?.title }}</button>
          <span>{{ module.files }} files</span><span>{{ module.lines.toLocaleString() }} lines</span>
        </div>
        <a v-if="module.docFile" class="ghost-link" :href="sourceUrl(data, module.docFile, 0, openIn)">{{ module.docFile }}</a>

        <section v-if="module.docHtml" class="inspector-section">
          <button class="section-toggle" @click="docOpen = !docOpen">
            <ChevronRight :size="14" :class="{ open: docOpen }" /> Full description
          </button>
          <div v-if="docOpen" class="doc-html" v-html="module.docHtml" />
        </section>

        <section v-if="moduleViews.length" class="inspector-section">
          <h3>Appears in</h3>
          <button v-for="other in moduleViews" :key="other.id" class="link-row" @click="emit('view', other.id)">
            <span class="kind-dot" :class="other.kind" />
            <span class="link-name">{{ other.title }}</span>
            <ChevronRight :size="13" class="link-chevron" />
          </button>
        </section>

        <section class="inspector-section">
          <h3>Symbols</h3>
          <button v-if="!symbols" class="ghost-button" @click="emit('loadSymbols')">Load symbols</button>
          <template v-else>
            <input v-model="symbolQuery" class="inspector-filter" placeholder="Filter" />
            <a
              v-for="symbol in moduleSymbols"
              :key="symbol.key"
              class="symbol-row"
              :href="sourceUrl(data, symbol.file, symbol.line, openIn)"
            >
              <span class="symbol-kind">{{ symbol.kind }}</span>
              <span class="symbol-name">{{ symbol.name }}</span>
            </a>
            <p v-if="!moduleSymbols.length" class="inspector-quiet">No exported symbols match.</p>
          </template>
        </section>
      </template>
    </template>

    <!-- View overview -->
    <template v-else-if="view">
      <div class="inspector-eyebrow">
        {{ KIND_LABEL[view.kind] }}<span v-if="view.derived"> · derived from source</span>
      </div>
      <h2 class="inspector-title">{{ view.title }}</h2>
      <p class="inspector-lede">{{ view.summary }}</p>
      <a v-if="!view.derived" class="ghost-link" :href="sourceUrl(data, view.source, 0, openIn)">{{ view.source }}</a>
      <AnchorCard v-if="view.enum" :data="data" :anchor="view.enum" :open-in="openIn" @module="emit('module', $event)" />

      <section v-for="(note, index) in view.notes ?? []" :key="note.title" class="inspector-section">
        <button class="section-toggle" @click="notesOpen = notesOpen === index ? null : index">
          <ChevronRight :size="14" :class="{ open: notesOpen === index }" /> {{ note.title }}
        </button>
        <div v-if="notesOpen === index" class="note-body">
          <p v-for="(paragraph, p) in paragraphs(note.body)" :key="p" v-html="paragraph" />
        </div>
      </section>

      <section v-if="view.related?.length" class="inspector-section">
        <h3>Related</h3>
        <button v-for="id in view.related" :key="id" class="link-row" @click="emit('view', id)">
          <span class="kind-dot" :class="views.get(id)?.kind" />
          <span class="link-name">{{ views.get(id)?.title ?? id }}</span>
          <ChevronRight :size="13" class="link-chevron" />
        </button>
      </section>
    </template>
  </aside>
</template>
