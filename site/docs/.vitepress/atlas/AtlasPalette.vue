<script setup lang="ts">
// ⌘K: one place to jump anywhere — views, modules, groups, symbols, actions.
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { Search, Boxes, Layers, Code2, Zap } from '@lucide/vue'
import { type AtlasData, type AtlasSymbol, KIND_LABEL, shortModule } from './model'

const props = defineProps<{ data: AtlasData; symbols: AtlasSymbol[] | null }>()
const emit = defineEmits<{
  close: []
  view: [id: string]
  module: [id: string]
  symbol: [symbol: AtlasSymbol]
  action: [id: string]
  loadSymbols: []
}>()

interface Item {
  type: 'view' | 'module' | 'group' | 'symbol' | 'action'
  id: string
  title: string
  detail: string
  kind?: string
  text: string
  weight: number
  symbol?: AtlasSymbol
}

const query = ref('')
const active = ref(0)
const input = ref<HTMLInputElement>()

const actions: Item[] = [
  { type: 'action', id: 'home', title: 'Go to briefing', detail: 'G H', text: 'home briefing overview', weight: 2 },
  { type: 'action', id: 'health', title: 'Open health', detail: 'problems and stale anchors', text: 'health problems stale', weight: 2 },
  { type: 'action', id: 'theme', title: 'Toggle light / dark', detail: 'appearance', text: 'theme dark light appearance', weight: 1 },
  { type: 'action', id: 'open-in', title: 'Switch source links (VS Code ↔ GitHub)', detail: 'open in', text: 'vscode github open source editor', weight: 1 },
]

const base = computed<Item[]>(() => [
  ...props.data.views
    .filter((view) => !view.id.startsWith('group-'))
    .map((view) => ({
      type: 'view' as const,
      id: view.id,
      title: view.title,
      detail: KIND_LABEL[view.kind],
      kind: view.kind,
      text: `${view.title} ${view.id} ${view.summary}`.toLowerCase(),
      weight: 4,
    })),
  ...props.data.groups.map((group) => ({
    type: 'group' as const,
    id: group.id,
    title: group.title,
    detail: `${group.side} group`,
    text: `${group.title} ${group.id} ${group.summary}`.toLowerCase(),
    weight: 3,
  })),
  ...props.data.modules.map((module) => ({
    type: 'module' as const,
    id: module.id,
    title: shortModule(module),
    detail: module.id,
    text: `${module.id} ${module.synopsis}`.toLowerCase(),
    weight: 2,
  })),
  ...actions,
])

const symbolItems = computed<Item[]>(() =>
  (props.symbols ?? []).map((symbol) => ({
    type: 'symbol' as const,
    id: symbol.key,
    title: symbol.name,
    detail: symbol.module.replace(/^(server\/internal|server|desktop\/internal|desktop|web\/src)\//, ''),
    text: symbol.name.toLowerCase(),
    weight: 1,
    symbol,
  })),
)

const results = computed<Item[]>(() => {
  const q = query.value.trim().toLowerCase()
  if (!q) return base.value.filter((item) => item.type === 'view' || item.type === 'action').slice(0, 14)
  const scored: { item: Item; score: number }[] = []
  for (const item of [...base.value, ...symbolItems.value]) {
    const at = item.text.indexOf(q)
    if (at === -1) continue
    const title = item.title.toLowerCase()
    const score = (title === q ? 20 : 0) + (title.startsWith(q) ? 8 : 0) + item.weight * 2 - at / 100
    scored.push({ item, score })
  }
  return scored
    .sort((a, b) => b.score - a.score)
    .slice(0, 40)
    .map((entry) => entry.item)
})

watch(query, (value) => {
  active.value = 0
  if (value.length > 1 && !props.symbols) emit('loadSymbols')
})

function choose(item: Item | undefined) {
  if (!item) return
  if (item.type === 'view') emit('view', item.id)
  else if (item.type === 'group') emit('view', `group-${item.id}`)
  else if (item.type === 'module') emit('module', item.id)
  else if (item.type === 'symbol' && item.symbol) emit('symbol', item.symbol)
  else emit('action', item.id)
  emit('close')
}

function onKey(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    active.value = Math.min(results.value.length - 1, active.value + 1)
  } else if (event.key === 'ArrowUp') {
    event.preventDefault()
    active.value = Math.max(0, active.value - 1)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    choose(results.value[active.value])
  } else if (event.key === 'Escape') {
    event.preventDefault()
    emit('close')
  }
}

watch(active, async () => {
  await nextTick()
  document.querySelector('.palette-item.active')?.scrollIntoView({ block: 'nearest' })
})

onMounted(() => input.value?.focus())
</script>

<template>
  <div class="palette-scrim" @pointerdown.self="emit('close')">
    <div class="palette" role="dialog" aria-label="Jump to">
      <div class="palette-input">
        <Search :size="16" />
        <input ref="input" v-model="query" placeholder="Jump to a view, module, group, or symbol" @keydown="onKey" />
        <kbd>esc</kbd>
      </div>
      <div class="palette-results">
        <button
          v-for="(item, index) in results"
          :key="`${item.type}:${item.id}`"
          class="palette-item"
          :class="{ active: index === active }"
          @pointerenter="active = index"
          @click="choose(item)"
        >
          <span class="palette-icon">
            <span v-if="item.type === 'view'" class="kind-dot" :class="item.kind" />
            <Layers v-else-if="item.type === 'group'" :size="14" />
            <Boxes v-else-if="item.type === 'module'" :size="14" />
            <Code2 v-else-if="item.type === 'symbol'" :size="14" />
            <Zap v-else :size="14" />
          </span>
          <span class="palette-title">{{ item.title }}</span>
          <span class="palette-detail">{{ item.detail }}</span>
        </button>
        <p v-if="!results.length" class="palette-empty">Nothing matches “{{ query }}”.</p>
      </div>
    </div>
  </div>
</template>
