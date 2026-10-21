<script setup lang="ts">
// The canvas renders one Mermaid diagram with the ELK layout and turns it into
// an explorable surface: pan, zoom, click to select, double-click to enter,
// and a spotlight that dims everything not connected to the selection.
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { mermaidId } from './model'

export interface CanvasEdge {
  from: string
  to: string
}

const props = defineProps<{
  source: string
  /** 'flow' (flowchart), 'state' (stateDiagram), or 'sequence'. */
  mode: 'flow' | 'state' | 'sequence'
  /** Node ids for flow/state, or "step:N" ids for sequence. */
  nodes: string[]
  edges: CanvasEdge[]
  selected?: string | null
  dark: boolean
  layout?: 'elk' | 'dagre'
  /** Pixels at the top of the viewport covered by the title overlay. */
  reserveTop?: number
}>()

const emit = defineEmits<{
  select: [id: string | null]
  open: [id: string]
}>()

const viewport = ref<HTMLDivElement>()
const stage = ref<HTMLDivElement>()
const error = ref('')
const zoom = ref(1)

let scale = 1
let x = 0
let y = 0
let renderToken = 0
const nodeEls = new Map<string, Element[]>()
const edgeEls: { from: string; to: string; els: Element[] }[] = []

let mermaidModule: typeof import('mermaid')['default'] | null = null

async function loadMermaid() {
  if (mermaidModule) return mermaidModule
  const [{ default: mermaid }, { default: elk }] = await Promise.all([
    import('mermaid'),
    import('@mermaid-js/layout-elk'),
  ])
  mermaid.registerLayoutLoaders(elk)
  mermaidModule = mermaid
  return mermaid
}

function token(name: string): string {
  return getComputedStyle(viewport.value!).getPropertyValue(name).trim()
}

async function render() {
  const current = ++renderToken
  const mermaid = await loadMermaid()
  // Mermaid measures label widths while rendering; measure with the real font.
  await Promise.all([
    document.fonts.load('13px "Geist Variable"'),
    document.fonts.load('600 13px "Geist Variable"'),
    document.fonts.load('11px "Geist Mono Variable"'),
  ]).catch(() => undefined)
  await document.fonts.ready
  const font = token('--atlas-font')
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: 'strict',
    theme: 'base',
    layout: props.layout ?? 'elk',
    elk: { mergeEdges: false, cycleBreakingStrategy: 'GREEDY_MODEL_ORDER', considerModelOrder: 'NODES_AND_EDGES' },
    fontFamily: font,
    themeVariables: {
      darkMode: props.dark,
      fontFamily: font,
      fontSize: '13px',
      background: 'transparent',
      primaryColor: token('--atlas-node'),
      primaryBorderColor: token('--atlas-node-line'),
      primaryTextColor: token('--atlas-ink'),
      secondaryColor: token('--atlas-node'),
      tertiaryColor: token('--atlas-node'),
      lineColor: token('--atlas-edge'),
      textColor: token('--atlas-ink-2'),
      mainBkg: token('--atlas-node'),
      nodeBorder: token('--atlas-node-line'),
      clusterBkg: 'transparent',
      clusterBorder: token('--atlas-line'),
      edgeLabelBackground: token('--atlas-canvas'),
      titleColor: token('--atlas-ink-3'),
      actorBkg: token('--atlas-node'),
      actorBorder: token('--atlas-node-line'),
      actorTextColor: token('--atlas-ink'),
      actorLineColor: token('--atlas-line'),
      signalColor: token('--atlas-edge'),
      signalTextColor: token('--atlas-ink-2'),
      labelBoxBkgColor: token('--atlas-node'),
      labelBoxBorderColor: token('--atlas-line'),
      labelTextColor: token('--atlas-ink-2'),
      loopTextColor: token('--atlas-ink-3'),
      noteBkgColor: token('--atlas-note'),
      noteBorderColor: token('--atlas-note-line'),
      noteTextColor: token('--atlas-ink-2'),
      sequenceNumberColor: token('--atlas-on-accent'),
      stateBkg: token('--atlas-node'),
      stateBorder: token('--atlas-node-line'),
      transitionColor: token('--atlas-edge'),
      transitionLabelColor: token('--atlas-ink-2'),
      specialStateColor: token('--atlas-ink-2'),
    },
    flowchart: { htmlLabels: true, curve: 'basis', padding: 14, nodeSpacing: 40, rankSpacing: 64 },
    state: { padding: 12 },
    sequence: { showSequenceNumbers: true, wrap: true, width: 170, actorMargin: 60, messageMargin: 34, mirrorActors: true, useMaxWidth: false },
    maxTextSize: 500000,
    maxEdges: 5000,
  })
  try {
    const { svg } = await mermaid.render(`atlas-${current}-${Date.now()}`, props.source)
    if (current !== renderToken || !stage.value) return
    stage.value.innerHTML = svg
    error.value = ''
  } catch (cause) {
    if (current !== renderToken) return
    error.value = String((cause as Error)?.message ?? cause)
    return
  }
  index()
  applySpotlight()
  await nextTick()
  fit(false)
  if (props.selected) focus(props.selected, false)
}

/** Maps rendered SVG elements back to view node ids and edges. */
function index() {
  nodeEls.clear()
  edgeEls.length = 0
  const svg = stage.value?.querySelector('svg')
  if (!svg) return
  if (props.mode === 'sequence') {
    // One line and one number per message; wrapped labels render as several
    // text elements, so each text joins the nearest message line below it.
    const lines = [...svg.querySelectorAll('[class^="messageLine"]')] as SVGGraphicsElement[]
    const numbers = [...svg.querySelectorAll('.sequenceNumber')]
    const tops = lines.map((line) => {
      const box = line.getBBox()
      return box.y
    })
    const groups: Element[][] = lines.map((line, index) => [line, ...(numbers[index] ? [numbers[index]] : [])])
    for (const text of svg.querySelectorAll('.messageText') as NodeListOf<SVGGraphicsElement>) {
      const bottom = text.getBBox().y + text.getBBox().height
      let best = -1
      let distance = Infinity
      tops.forEach((top, index) => {
        const gap = top - bottom
        if (gap >= -6 && gap < distance) {
          distance = gap
          best = index
        }
      })
      if (best >= 0) groups[best].push(text)
    }
    groups.forEach((els, index) => {
      const id = `step:${index + 1}`
      els.forEach((el) => el.setAttribute('data-atlas', id))
      nodeEls.set(id, els)
    })
    return
  }
  const byMermaid = new Map(props.nodes.map((id) => [mermaidId(id), id]))
  // Mermaid ids look like "<render id>-flowchart-n_web-3" or "...-state-n_ready-2".
  const pattern = /(?:flowchart|state)-(n_[A-Za-z0-9_]+?)-\d+$/
  for (const el of svg.querySelectorAll('g.node')) {
    const key = el.getAttribute('data-id') ?? el.id
    const match = pattern.exec(key)?.[1] ?? (byMermaid.has(key) ? key : undefined)
    const id = match ? byMermaid.get(match) : undefined
    if (!id) continue
    el.setAttribute('data-atlas', id)
    nodeEls.set(id, [...(nodeEls.get(id) ?? []), el])
  }
  const paths = [...svg.querySelectorAll('path[data-edge="true"], path.flowchart-link, path.transition')]
  const labels = [...svg.querySelectorAll('g.edgeLabel')]
  props.edges.forEach((edge, index) => {
    const from = mermaidId(edge.from)
    const to = mermaidId(edge.to)
    const prefix = `L_${from}_${to}_`
    const els = paths.filter((path) => (path.getAttribute('data-id') ?? path.id.replace(/^.*?-(L_)/, '$1')).startsWith(prefix))
    const chosen = els.length ? els : paths[index] ? [paths[index]] : []
    const label = labels.find((el) => el.querySelector(`[data-id^="${prefix}"]`))
    edgeEls.push({ from: edge.from, to: edge.to, els: [...chosen, ...(label ? [label] : [])] })
  })
}

function applySpotlight() {
  const svg = stage.value?.querySelector('svg')
  if (!svg) return
  svg.querySelectorAll('.atlas-lit, .atlas-picked').forEach((el) => el.classList.remove('atlas-lit', 'atlas-picked'))
  const selected = props.selected
  const els = selected ? nodeEls.get(selected) : undefined
  svg.classList.toggle('atlas-dim', Boolean(els?.length))
  if (!selected || !els?.length) return
  els.forEach((el) => el.classList.add('atlas-picked', 'atlas-lit'))
  if (props.mode === 'sequence') return
  for (const edge of edgeEls) {
    if (edge.from !== selected && edge.to !== selected) continue
    edge.els.forEach((el) => el.classList.add('atlas-lit'))
    const other = edge.from === selected ? edge.to : edge.from
    nodeEls.get(other)?.forEach((el) => el.classList.add('atlas-lit'))
  }
}

function apply(animate: boolean) {
  if (!stage.value) return
  stage.value.classList.toggle('animating', animate)
  stage.value.style.transform = `translate(${x}px, ${y}px) scale(${scale})`
  // The dot grid moves and scales with the map, so panning feels physical.
  if (viewport.value) {
    const grid = Math.max(10, 22 * scale)
    viewport.value.style.backgroundSize = `${grid}px ${grid}px`
    viewport.value.style.backgroundPosition = `${x}px ${y}px`
  }
  zoom.value = scale
}

function fit(animate = true) {
  const svg = stage.value?.querySelector('svg')
  const room = viewport.value?.getBoundingClientRect()
  if (!svg || !room) return
  const width = svg.viewBox.baseVal?.width || svg.getBoundingClientRect().width / scale
  const height = svg.viewBox.baseVal?.height || svg.getBoundingClientRect().height / scale
  svg.setAttribute('width', String(width))
  svg.setAttribute('height', String(height))
  svg.style.maxWidth = 'none'
  const top = props.reserveTop ?? 0
  const pad = 56
  scale = Math.min(1.2, (room.width - pad * 2) / width, (room.height - top - pad * 1.5) / height)
  x = (room.width - width * scale) / 2
  y = top + Math.max(pad * 0.4, (room.height - top - pad - height * scale) / 2)
  apply(animate)
}

/** The selection plus every node one hop away: what focus should frame. */
function neighbourhood(id: string): Element[] {
  const els = [...(nodeEls.get(id) ?? [])]
  if (props.mode === 'sequence') return els
  for (const edge of edgeEls) {
    if (edge.from === id) els.push(...(nodeEls.get(edge.to) ?? []))
    if (edge.to === id) els.push(...(nodeEls.get(edge.from) ?? []))
  }
  return els
}

function focus(id: string, animate = true) {
  const els = neighbourhood(id)
  const room = viewport.value?.getBoundingClientRect()
  if (!els?.length || !room || !stage.value) return
  const boxes = els.map((el) => el.getBoundingClientRect())
  const left = Math.min(...boxes.map((box) => box.left))
  const top = Math.min(...boxes.map((box) => box.top))
  const right = Math.max(...boxes.map((box) => box.right))
  const bottom = Math.max(...boxes.map((box) => box.bottom))
  // Convert to stage coordinates, then frame the element in the viewport.
  const sx = (left - room.left - x) / scale
  const sy = (top - room.top - y) / scale
  const sw = (right - left) / scale
  const sh = (bottom - top) / scale
  // Keep context: never zoom past 100%, lift tiny maps to a readable 70%,
  // and only shrink when the selection would not fit.
  const reserved = props.reserveTop ?? 0
  let next = Math.min(1, Math.max(scale, 0.7))
  next = Math.max(0.3, Math.min(next, (room.width * 0.86) / sw, ((room.height - reserved) * 0.84) / sh))
  scale = next
  x = room.width / 2 - (sx + sw / 2) * scale
  y = reserved + (room.height - reserved) / 2 - (sy + sh / 2) * scale
  apply(animate)
}

function zoomBy(factor: number, cx?: number, cy?: number, animate = false) {
  const room = viewport.value!.getBoundingClientRect()
  const px = (cx ?? room.left + room.width / 2) - room.left
  const py = (cy ?? room.top + room.height / 2) - room.top
  const next = Math.min(3, Math.max(0.12, scale * factor))
  x = px - ((px - x) * next) / scale
  y = py - ((py - y) * next) / scale
  scale = next
  apply(animate)
}

function onWheel(event: WheelEvent) {
  event.preventDefault()
  if (event.ctrlKey || event.metaKey) {
    zoomBy(Math.exp(-event.deltaY * 0.01), event.clientX, event.clientY)
  } else {
    x -= event.deltaX
    y -= event.deltaY
    apply(false)
  }
}

let drag: { x: number; y: number; moved: boolean } | null = null
function onPointerDown(event: PointerEvent) {
  if (event.button !== 0) return
  drag = { x: event.clientX - x, y: event.clientY - y, moved: false }
}
function onPointerMove(event: PointerEvent) {
  if (!drag) return
  const nx = event.clientX - drag.x
  const ny = event.clientY - drag.y
  if (!drag.moved && Math.hypot(nx - x, ny - y) < 4) return
  if (!drag.moved) viewport.value?.setPointerCapture(event.pointerId)
  drag.moved = true
  viewport.value?.classList.add('dragging')
  x = nx
  y = ny
  apply(false)
}
function onPointerUp(event: PointerEvent) {
  const moved = drag?.moved
  drag = null
  viewport.value?.classList.remove('dragging')
  if (moved) return
  const hit = (event.target as Element).closest?.('[data-atlas]')
  emit('select', hit ? hit.getAttribute('data-atlas') : null)
}
function onDoubleClick(event: MouseEvent) {
  const hit = (event.target as Element).closest?.('[data-atlas]')
  if (hit) emit('open', hit.getAttribute('data-atlas')!)
}

defineExpose({ fit, zoomBy, focus })

watch(() => [props.source, props.dark, props.layout], render)
watch(
  () => props.selected,
  (id) => {
    applySpotlight()
    if (id) focus(id)
  },
)

let resize: ResizeObserver | undefined
onMounted(() => {
  render()
  resize = new ResizeObserver(() => fit(false))
  if (viewport.value) resize.observe(viewport.value)
})
onBeforeUnmount(() => resize?.disconnect())
</script>

<template>
  <div
    ref="viewport"
    class="atlas-canvas"
    :class="`mode-${mode}`"
    @wheel="onWheel"
    @pointerdown="onPointerDown"
    @pointermove="onPointerMove"
    @pointerup="onPointerUp"
    @dblclick="onDoubleClick"
  >
    <div ref="stage" class="atlas-stage" />
    <pre v-if="error" class="atlas-canvas-error">{{ error }}</pre>
    <div class="atlas-zoom" @pointerdown.stop @pointerup.stop @dblclick.stop>
      <button title="Zoom out" @click="zoomBy(0.8, undefined, undefined, true)">−</button>
      <button class="atlas-zoom-level" title="Fit (F)" @click="fit()">{{ Math.round(zoom * 100) }}%</button>
      <button title="Zoom in" @click="zoomBy(1.25, undefined, undefined, true)">+</button>
    </div>
  </div>
</template>
