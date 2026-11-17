// Types and pure helpers for the Atlas site. The model is produced by
// server/tools/atlas (`atlas data`); nothing here re-derives architecture.

export type Kind = 'architecture' | 'sequence' | 'dataflow' | 'lifecycle'

export interface AtlasNode {
  id: string
  label: string
  anchor?: string
  shape?: string
  group?: string
  note?: string
  member?: string
}

export interface AtlasEdge {
  from: string
  to: string
  label?: string
  anchor?: string
  style?: string
}

export interface SequenceRow {
  number: number
  from: string
  to: string
  label: string
  anchor?: string
  block?: string
}

export interface AtlasView {
  id: string
  kind: Kind
  category: Kind
  title: string
  summary: string
  direction?: string
  /** Layout engine: ELK by default; dagre suits some cyclic state machines. */
  layout?: 'elk' | 'dagre'
  enum?: string
  nodes: AtlasNode[]
  edges?: AtlasEdge[]
  transitions?: AtlasEdge[]
  notes?: { title: string; body: string }[]
  related?: string[]
  source: string
  derived: boolean
  mermaid: string
  rows?: SequenceRow[]
}

export interface AtlasModule {
  id: string
  side: 'server' | 'desktop' | 'web'
  kind: string
  name: string
  importPath?: string
  group: string
  synopsis: string
  docHtml: string
  docFile?: string
  files: number
  lines: number
  imports: string[]
  importedBy: string[]
}

export interface AtlasGroup {
  id: string
  title: string
  side: 'server' | 'desktop' | 'web'
  summary: string
  uses?: string[]
  layer: number
}

export interface ResolvedAnchor {
  anchor: string
  kind: string
  label: string
  module?: string
  file?: string
  line?: number
  stale?: boolean
}

export interface Problem {
  where: string
  message: string
}

export interface AtlasData {
  repo: { root: string; github: string; branch: string }
  groups: AtlasGroup[]
  modules: AtlasModule[]
  views: AtlasView[]
  anchors: Record<string, ResolvedAnchor>
  usage: Record<string, string[]>
  problems: Problem[]
  stale: Problem[]
}

export interface AtlasSymbol {
  key: string
  name: string
  module: string
  file: string
  line: number
  kind: string
}

export type Route =
  | { page: 'home' }
  | { page: 'health' }
  | { page: 'view'; id: string; select?: string }
  | { page: 'module'; id: string }

export const KINDS: Kind[] = ['architecture', 'sequence', 'dataflow', 'lifecycle']

export const KIND_LABEL: Record<Kind, string> = {
  architecture: 'Architecture',
  sequence: 'Sequence',
  dataflow: 'Data flow',
  lifecycle: 'Lifecycle',
}

export const SIDE_LABEL = { server: 'Server', desktop: 'Desktop', web: 'Web' } as const

/** Mirrors mermaidID in server/tools/atlas/views.go. */
export function mermaidId(id: string): string {
  return `n_${id.replace(/[^A-Za-z0-9_]/g, '_')}`
}

export function parseRoute(hash: string): Route {
  const raw = decodeURIComponent(hash.replace(/^#\/?/, ''))
  const [path, query = ''] = raw.split('?')
  const params = new URLSearchParams(query)
  const [head, ...rest] = path.split('/')
  const id = rest.join('/')
  if (head === 'v' && id) return { page: 'view', id, select: params.get('s') ?? undefined }
  if (head === 'm' && id) return { page: 'module', id }
  if (head === 'health') return { page: 'health' }
  return { page: 'home' }
}

export function routeHash(route: Route): string {
  switch (route.page) {
    case 'view':
      return `#/v/${route.id}${route.select ? `?s=${encodeURIComponent(route.select)}` : ''}`
    case 'module':
      return `#/m/${route.id}`
    case 'health':
      return '#/health'
    default:
      return '#/'
  }
}

export function shortModule(module: Pick<AtlasModule, 'id' | 'side'>): string {
  const parts = module.id.split('/')
  if (module.side === 'web') return parts.slice(2).join('/') || module.id
  if (parts[1] === 'internal') return parts.slice(2).join('/')
  return parts.slice(1).join('/') || module.id
}

export function firstSentence(text: string): string {
  const trimmed = text.trim()
  const end = trimmed.search(/\.\s/)
  return end > 0 ? trimmed.slice(0, end + 1) : trimmed
}

export function sourceUrl(data: AtlasData, file: string, line: number | undefined, target: 'vscode' | 'github'): string {
  if (target === 'github') {
    return `${data.repo.github}/blob/${data.repo.branch}/${file}${line ? `#L${line}` : ''}`
  }
  return `vscode://file/${data.repo.root}/${file}${line ? `:${line}` : ''}`
}

/** A neighbourhood graph for one module, laid out importers → module → imports. */
export function moduleMermaid(data: AtlasData, module: AtlasModule): string {
  const byId = new Map(data.modules.map((item) => [item.id, item]))
  const label = (id: string) => {
    const item = byId.get(id)
    return (item ? shortModule(item) : id).replace(/"/g, '#quot;')
  }
  const lines = ['flowchart LR']
  if (module.importedBy.length) {
    lines.push(`  subgraph ${mermaidId('in')}["Used by"]`)
    for (const id of module.importedBy) lines.push(`    ${mermaidId(id)}["${label(id)}"]`)
    lines.push('  end')
  }
  lines.push(`  ${mermaidId(module.id)}["${label(module.id)}"]`)
  if (module.imports.length) {
    lines.push(`  subgraph ${mermaidId('out')}["Uses"]`)
    for (const id of module.imports) lines.push(`    ${mermaidId(id)}["${label(id)}"]`)
    lines.push('  end')
  }
  for (const id of module.importedBy) lines.push(`  ${mermaidId(id)} --> ${mermaidId(module.id)}`)
  for (const id of module.imports) lines.push(`  ${mermaidId(module.id)} --> ${mermaidId(id)}`)
  return lines.join('\n')
}
