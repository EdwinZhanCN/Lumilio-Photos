<script setup lang="ts">
// Lumilio Atlas — a calm instrument for exploring the codebase one node at a
// time. The canvas is the room; the navigator, trail, and inspector float on
// it, and text appears only when asked for.
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useData } from 'vitepress'
import {
  Network,
  ArrowLeftRight,
  Workflow,
  CircleDot,
  Boxes,
  Activity,
  Sun,
  Moon,
  Search,
  PanelLeft,
  Info,
  ChevronRight,
  Compass,
} from '@lucide/vue'
import '@fontsource-variable/geist'
import '@fontsource-variable/geist-mono'
import './atlas.css'
import AtlasCanvas from './AtlasCanvas.vue'
import AtlasInspector from './AtlasInspector.vue'
import AtlasPalette from './AtlasPalette.vue'
import loaded from 'virtual:lumilio-atlas'
import {
  type AtlasData,
  type AtlasSymbol,
  type AtlasView,
  type Kind,
  type Route,
  KINDS,
  KIND_LABEL,
  SIDE_LABEL,
  firstSentence,
  moduleMermaid,
  parseRoute,
  routeHash,
  shortModule,
} from './model'

const data = loaded as AtlasData | null
const { isDark } = useData()

type Lens = Kind | 'modules'
const LENS_ICON = { architecture: Network, sequence: ArrowLeftRight, dataflow: Workflow, lifecycle: CircleDot, modules: Boxes }

function stored<T extends string>(key: string, fallback: T): T {
  try {
    return (localStorage.getItem(key) as T) ?? fallback
  } catch {
    return fallback
  }
}
function store(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // Storage may be unavailable; preferences then last for the session.
  }
}

const route = ref<Route>(parseRoute(location.hash))
const lens = ref<Lens>('architecture')
const navOpen = ref(stored('atlas.nav', 'open') === 'open')
const inspectorOpen = ref(false)
const paletteOpen = ref(false)
const openIn = ref<'vscode' | 'github'>(stored('atlas.openIn', 'vscode'))
const symbols = shallowRef<AtlasSymbol[] | null>(null)
const canvas = ref<InstanceType<typeof AtlasCanvas>>()
const titleEl = ref<HTMLElement>()
const titleHeight = ref(160)
let titleObserver: ResizeObserver | undefined

interface TrailStop {
  hash: string
  label: string
  kind: string
}
const trail = ref<TrailStop[]>([])
const recent = ref<TrailStop[]>(JSON.parse(stored('atlas.recent', '[]')))

const views = computed(() => new Map((data?.views ?? []).map((view) => [view.id, view])))
const modules = computed(() => new Map((data?.modules ?? []).map((module) => [module.id, module])))
const groups = computed(() => new Map((data?.groups ?? []).map((group) => [group.id, group])))

const view = computed(() => (route.value.page === 'view' ? views.value.get(route.value.id) : undefined))
const module = computed(() => (route.value.page === 'module' ? modules.value.get(route.value.id) : undefined))
const selected = computed(() => (route.value.page === 'view' ? (route.value.select ?? null) : moduleSelection.value))
const moduleSelection = ref<string | null>(null)

const staleViews = computed(() => {
  const set = new Set<string>()
  for (const item of data?.stale ?? []) {
    const match = data?.views.find((candidate) => candidate.source === item.where)
    if (match) set.add(match.id)
  }
  return set
})
const healthy = computed(() => (data?.problems.length ?? 0) + (data?.stale.length ?? 0) === 0)

const canvasInput = computed(() => {
  if (view.value) {
    const current = view.value
    if (current.kind === 'sequence') {
      return {
        source: current.mermaid,
        mode: 'sequence' as const,
        nodes: (current.rows ?? []).map((row) => `step:${row.number}`),
        edges: [],
      }
    }
    return {
      source: current.mermaid,
      mode: current.kind === 'lifecycle' ? ('state' as const) : ('flow' as const),
      nodes: current.nodes.map((node) => node.id),
      edges: (current.edges ?? current.transitions ?? []).map((edge) => ({ from: edge.from, to: edge.to })),
    }
  }
  if (module.value && data) {
    const current = module.value
    return {
      source: moduleMermaid(data, current),
      mode: 'flow' as const,
      nodes: [current.id, ...current.imports, ...current.importedBy],
      edges: [
        ...current.importedBy.map((id) => ({ from: id, to: current.id })),
        ...current.imports.map((id) => ({ from: current.id, to: id })),
      ],
    }
  }
  return null
})

// ----------------------------------------------------------------- routing

function go(next: Route, replace = false) {
  const hash = routeHash(next)
  if (replace) history.replaceState(null, '', hash)
  else history.pushState(null, '', hash)
  onRoute()
}

function onRoute() {
  route.value = parseRoute(location.hash)
  moduleSelection.value = null
  const current = route.value
  if (current.page === 'view') {
    const target = views.value.get(current.id)
    if (target) {
      lens.value = target.kind
      remember({ hash: routeHash({ page: 'view', id: target.id }), label: target.title, kind: target.kind })
    }
    inspectorOpen.value = Boolean(current.select)
  } else if (current.page === 'module') {
    const target = modules.value.get(current.id)
    if (target) {
      lens.value = 'modules'
      remember({ hash: routeHash(current), label: shortModule(target), kind: 'module' })
    }
    inspectorOpen.value = true
  }
}

function remember(stop: TrailStop) {
  const at = trail.value.findIndex((item) => item.hash === stop.hash)
  trail.value = at >= 0 ? trail.value.slice(0, at + 1) : [...trail.value, stop].slice(-6)
  recent.value = [stop, ...recent.value.filter((item) => item.hash !== stop.hash)].slice(0, 8)
  store('atlas.recent', JSON.stringify(recent.value))
}

function openView(id: string, select?: string) {
  go({ page: 'view', id, select })
}
function openModule(id: string) {
  go({ page: 'module', id })
}
function select(id: string | null) {
  if (route.value.page === 'view') {
    go({ page: 'view', id: route.value.id, select: id ?? undefined }, true)
    inspectorOpen.value = Boolean(id) || inspectorOpen.value
  } else if (route.value.page === 'module') {
    moduleSelection.value = id && id !== route.value.id ? id : null
  }
}
function enter(id: string) {
  if (route.value.page === 'module') {
    if (modules.value.has(id)) openModule(id)
    return
  }
  const node = view.value?.nodes.find((item) => item.id === id)
  const anchor = node?.anchor ?? ''
  if (anchor.startsWith('group:')) openView(`group-${anchor.slice(6)}`)
  else if (anchor.startsWith('mod:')) openModule(data!.anchors[anchor]?.module ?? anchor.slice(4))
  else select(id)
}

function step(delta: number) {
  const current = view.value
  if (!current) return
  const ids = current.kind === 'sequence' ? (current.rows ?? []).map((row) => `step:${row.number}`) : current.nodes.map((node) => node.id)
  if (!ids.length) return
  const at = selected.value ? ids.indexOf(selected.value) : -1
  const next = ids[Math.min(ids.length - 1, Math.max(0, at + delta))]
  select(at === -1 ? ids[0] : next)
}

async function loadSymbols() {
  if (symbols.value) return
  const module = await import('virtual:lumilio-atlas-symbols')
  symbols.value = module.default
}

function onAction(id: string) {
  if (id === 'home') go({ page: 'home' })
  if (id === 'health') go({ page: 'health' })
  if (id === 'theme') isDark.value = !isDark.value
  if (id === 'open-in') {
    openIn.value = openIn.value === 'vscode' ? 'github' : 'vscode'
    store('atlas.openIn', openIn.value)
  }
}

function toggleNav() {
  navOpen.value = !navOpen.value
  store('atlas.nav', navOpen.value ? 'open' : 'closed')
}

// --------------------------------------------------------------- keyboard

function onKey(event: KeyboardEvent) {
  const typing = (event.target as HTMLElement)?.closest?.('input, textarea')
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    paletteOpen.value = !paletteOpen.value
    return
  }
  if (paletteOpen.value || typing) return
  switch (event.key) {
    case '/':
      event.preventDefault()
      paletteOpen.value = true
      break
    case 'Escape':
      if (selected.value) select(null)
      else inspectorOpen.value = false
      break
    case 'j':
    case 'ArrowDown':
      if (canvasInput.value) {
        event.preventDefault()
        step(1)
      }
      break
    case 'k':
    case 'ArrowUp':
      if (canvasInput.value) {
        event.preventDefault()
        step(-1)
      }
      break
    case 'Enter':
      if (selected.value) enter(selected.value)
      break
    case 'f':
      canvas.value?.fit()
      break
    case 'i':
      inspectorOpen.value = !inspectorOpen.value
      break
    case '[':
      toggleNav()
      break
    case 'Backspace':
      history.back()
      break
    case '1':
    case '2':
    case '3':
    case '4':
      lens.value = KINDS[Number(event.key) - 1]
      navOpen.value = true
      break
    case '5':
      lens.value = 'modules'
      navOpen.value = true
      break
  }
}

onMounted(() => {
  titleObserver = new ResizeObserver(() => {
    if (titleEl.value) titleHeight.value = titleEl.value.offsetTop + titleEl.value.offsetHeight + 8
  })
  watch(titleEl, (el) => el && titleObserver?.observe(el), { immediate: true })
  addEventListener('popstate', onRoute)
  addEventListener('hashchange', onRoute)
  addEventListener('keydown', onKey)
  onRoute()
})
onBeforeUnmount(() => {
  titleObserver?.disconnect()
  removeEventListener('popstate', onRoute)
  removeEventListener('hashchange', onRoute)
  removeEventListener('keydown', onKey)
})
watch(isDark, () => document.documentElement.classList.toggle('dark', isDark.value))

// -------------------------------------------------------------- navigator

const lensViews = computed(() =>
  lens.value === 'modules' ? [] : (data?.views ?? []).filter((item) => item.category === lens.value && !item.id.startsWith('group-')),
)
const groupViews = (side: string) =>
  (data?.groups ?? [])
    .filter((group) => group.side === side)
    .sort((a, b) => b.layer - a.layer)
    .map((group) => views.value.get(`group-${group.id}`))
    .filter(Boolean) as AtlasView[]
const openGroup = ref<string | null>(null)
const lensCount = (kind: Lens) =>
  kind === 'modules' ? data?.modules.length ?? 0 : (data?.views ?? []).filter((item) => item.category === kind && !item.id.startsWith('group-')).length

const briefing = computed(() =>
  KINDS.map((kind) => ({
    kind,
    views: (data?.views ?? []).filter((item) => item.category === kind && !item.id.startsWith('group-')),
  })),
)
const totals = computed(() => ({
  modules: data?.modules.length ?? 0,
  lines: Math.round((data?.modules ?? []).reduce((sum, item) => sum + item.lines, 0) / 1000),
  anchors: Object.keys(data?.anchors ?? {}).length,
  views: (data?.views ?? []).filter((item) => !item.derived).length,
}))
</script>

<template>
  <div class="atlas-root" :class="{ dark: isDark }" :style="{ '--accent': `var(--lens-${view?.kind ?? 'architecture'})` }">
    <div v-if="!data" class="atlas-empty">
      <Compass :size="28" />
      <h1>No Atlas data yet</h1>
      <p>Run <code>task atlas</code> from the repository root; it extracts the model and opens this page.</p>
    </div>

    <template v-else>
      <!-- Rail -->
      <nav class="rail">
        <a class="rail-mark" href="#/" title="Briefing"><span class="mark" /></a>
        <button
          v-for="(icon, key) in LENS_ICON"
          :key="key"
          class="rail-button"
          :class="[{ active: lens === key && navOpen }, `lens-${key}`]"
          :title="key === 'modules' ? 'Modules' : KIND_LABEL[key as Kind]"
          @click="lens === key ? toggleNav() : ((lens = key as Lens), (navOpen = true))"
        >
          <component :is="icon" :size="18" :stroke-width="1.75" />
        </button>
        <span class="rail-spacer" />
        <a class="rail-button" :class="{ alert: !healthy }" href="#/health" title="Health">
          <Activity :size="18" :stroke-width="1.75" />
        </a>
        <button class="rail-button" title="Light / dark" @click="isDark = !isDark">
          <Moon v-if="!isDark" :size="17" :stroke-width="1.75" /><Sun v-else :size="17" :stroke-width="1.75" />
        </button>
      </nav>

      <!-- Navigator -->
      <aside v-if="navOpen" class="navigator">
        <header class="navigator-head">
          <span>{{ lens === 'modules' ? 'Modules' : KIND_LABEL[lens as Kind] }}</span>
          <span class="count">{{ lensCount(lens) }}</span>
        </header>
        <div class="navigator-list">
          <a
            v-for="item in lensViews"
            :key="item.id"
            class="nav-item"
            :class="{ active: view?.id === item.id }"
            :href="`#/v/${item.id}`"
          >
            <span class="kind-dot" :class="item.kind" />
            <span class="nav-label">{{ item.title }}</span>
            <span v-if="staleViews.has(item.id)" class="nav-flag" title="Needs re-verification" />
          </a>
          <template v-if="lens === 'architecture'">
            <div v-for="side in ['server', 'desktop', 'web']" :key="side" class="nav-section">
              <div class="nav-section-title">{{ SIDE_LABEL[side as 'server'] }} groups</div>
              <a
                v-for="item in groupViews(side)"
                :key="item.id"
                class="nav-item compact"
                :class="{ active: view?.id === item.id }"
                :href="`#/v/${item.id}`"
              >
                <span class="nav-label">{{ item.title }}</span>
              </a>
            </div>
          </template>
          <template v-if="lens === 'modules'">
            <div v-for="group in [...data.groups].sort((a, b) => a.side.localeCompare(b.side) || b.layer - a.layer)" :key="group.id" class="nav-section">
              <button class="nav-section-title toggle" @click="openGroup = openGroup === group.id ? null : group.id">
                <ChevronRight :size="12" :class="{ open: openGroup === group.id || module?.group === group.id }" />
                {{ SIDE_LABEL[group.side] }} · {{ group.title }}
              </button>
              <template v-if="openGroup === group.id || module?.group === group.id">
                <a
                  v-for="item in data.modules.filter((candidate) => candidate.group === group.id)"
                  :key="item.id"
                  class="nav-item compact mono"
                  :class="{ active: module?.id === item.id }"
                  :href="`#/m/${item.id}`"
                >
                  <span class="nav-label">{{ shortModule(item) }}</span>
                </a>
              </template>
            </div>
          </template>
        </div>
      </aside>

      <!-- Main -->
      <main class="stage-area">
        <header class="topbar">
          <button v-if="!navOpen" class="icon-button" title="Show navigator ([)" @click="toggleNav"><PanelLeft :size="16" /></button>
          <nav class="trail">
            <a href="#/" class="trail-stop home">Atlas</a>
            <template v-for="stop in trail" :key="stop.hash">
              <ChevronRight :size="13" class="trail-sep" />
              <a :href="stop.hash" class="trail-stop" :class="{ current: stop.hash === routeHash({ ...route, select: undefined } as Route) }">
                <span v-if="stop.kind !== 'module'" class="kind-dot" :class="stop.kind" />{{ stop.label }}
              </a>
            </template>
          </nav>
          <button class="jump" @click="paletteOpen = true">
            <Search :size="14" /> <span>Jump to…</span> <kbd>⌘K</kbd>
          </button>
          <a class="status" :class="healthy ? 'ok' : 'bad'" href="#/health">
            <span class="status-dot" />{{ healthy ? 'In sync' : `${data.problems.length + data.stale.length} to review` }}
          </a>
        </header>

        <!-- Canvas pages -->
        <section v-if="canvasInput" class="canvas-wrap">
          <div ref="titleEl" class="canvas-title" :class="{ compact: Boolean(selected) }">
            <template v-if="view">
              <span class="kind-chip" :class="view.kind"><span class="kind-dot" :class="view.kind" />{{ KIND_LABEL[view.kind] }}</span>
              <h1>{{ view.title }}</h1>
              <p>{{ firstSentence(view.summary) }}</p>
            </template>
            <template v-else-if="module">
              <span class="kind-chip"><Boxes :size="12" /> {{ groups.get(module.group)?.title }}</span>
              <h1 class="mono">{{ shortModule(module) }}</h1>
              <p>{{ module.synopsis }}</p>
            </template>
            <button v-if="!inspectorOpen" class="about" @click="inspectorOpen = true"><Info :size="13" /> About</button>
          </div>
          <AtlasCanvas
            ref="canvas"
            :key="view?.id ?? module?.id"
            :source="canvasInput.source"
            :mode="canvasInput.mode"
            :nodes="canvasInput.nodes"
            :edges="canvasInput.edges"
            :selected="selected"
            :dark="isDark"
            :layout="view?.layout"
            :reserve-top="titleHeight"
            @select="select"
            @open="enter"
          />
          <div class="hint"><kbd>J</kbd><kbd>K</kbd> walk · <kbd>↵</kbd> enter · <kbd>F</kbd> fit · <kbd>⌫</kbd> back</div>
        </section>

        <!-- Briefing -->
        <section v-else-if="route.page === 'home'" class="page briefing">
          <div class="briefing-head">
            <span class="mark large" />
            <div>
              <h1>Lumilio Atlas</h1>
              <p class="status-line" :class="healthy ? 'ok' : 'bad'">
                <span class="status-dot" />
                {{ healthy ? 'Every view matches the code.' : `${data.problems.length} problems · ${data.stale.length} anchors to re-verify` }}
              </p>
            </div>
          </div>
          <div class="totals">
            <div><b>{{ totals.modules }}</b><span>modules</span></div>
            <div><b>{{ totals.lines }}k</b><span>lines mapped</span></div>
            <div><b>{{ totals.views }}</b><span>authored views</span></div>
            <div><b>{{ totals.anchors }}</b><span>anchors</span></div>
          </div>

          <a class="start" href="#/v/system-context">
            <Network :size="18" /> <span><b>Start at the top</b> System context</span> <ChevronRight :size="16" />
          </a>

          <div v-if="recent.length" class="recent">
            <h2>Continue</h2>
            <a v-for="stop in recent.slice(0, 5)" :key="stop.hash" :href="stop.hash" class="recent-item">
              <span v-if="stop.kind !== 'module'" class="kind-dot" :class="stop.kind" /><Boxes v-else :size="13" />
              {{ stop.label }}
            </a>
          </div>

          <div class="lenses">
            <div v-for="lensGroup in briefing" :key="lensGroup.kind" class="lens-card" :class="lensGroup.kind">
              <header>
                <component :is="LENS_ICON[lensGroup.kind]" :size="16" :stroke-width="1.75" />
                {{ KIND_LABEL[lensGroup.kind] }}
                <span class="count">{{ lensGroup.views.length }}</span>
              </header>
              <a v-for="item in lensGroup.views" :key="item.id" :href="`#/v/${item.id}`" class="lens-item">
                {{ item.title }}
                <span v-if="staleViews.has(item.id)" class="nav-flag" />
              </a>
            </div>
          </div>
        </section>

        <!-- Health -->
        <section v-else-if="route.page === 'health'" class="page health">
          <h1>Health</h1>
          <p class="page-lede">
            Problems are broken anchors, undeclared dependencies, or missing package docs. A stale anchor means its code
            changed after the view was verified: re-read the view, fix it if behaviour changed, then run
            <code>task atlas:lock</code>.
          </p>
          <div v-if="healthy" class="health-ok"><span class="status-dot" /> Nothing to review.</div>
          <template v-for="(list, title) in { Problems: data.problems, 'Stale anchors': data.stale }" :key="title">
            <h2 v-if="list.length">{{ title }} <span class="count">{{ list.length }}</span></h2>
            <div v-for="(item, index) in list" :key="index" class="health-row">
              <code>{{ item.where }}</code>
              <span>{{ item.message }}</span>
            </div>
          </template>
        </section>
      </main>

      <AtlasInspector
        v-if="inspectorOpen && (view || module)"
        :data="data"
        :view="view"
        :module="module"
        :selected="selected"
        :open-in="openIn"
        :symbols="symbols"
        @close="inspectorOpen = false"
        @select="select"
        @view="openView"
        @module="openModule"
        @step="step"
        @load-symbols="loadSymbols"
      />

      <AtlasPalette
        v-if="paletteOpen"
        :data="data"
        :symbols="symbols"
        @close="paletteOpen = false"
        @view="openView"
        @module="openModule"
        @symbol="(symbol) => openModule(symbol.module)"
        @action="onAction"
        @load-symbols="loadSymbols"
      />
    </template>
  </div>
</template>
