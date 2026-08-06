/**
 * Documentation site CI checks.
 *
 * Runs fast, dependency-free checks that complement the VitePress build
 * (which already fails on dead internal links and anchors):
 *
 * 1. Key bilingual pages exist in both `en` and `zh-cn`.
 * 2. Forbidden product names never appear in public docs:
 *    "Lumen AI", "Lumen ML", "LumenAI".
 * 3. The removed "overwrite" conflict policy never appears.
 * 4. Only published presets are documented (minimal, basic, brave, custom).
 * 5. Internal markdown links resolve to existing files, and fragments
 *    resolve to existing heading slugs.
 *
 * Internal engineering docs under `internal/**` are excluded from the public
 * build and therefore from these checks.
 */
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, normalize, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const root = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'docs')

const FORBIDDEN = [
  { pattern: /\bLumen AI\b/g, label: 'Lumen AI' },
  { pattern: /\bLumen ML\b/g, label: 'Lumen ML' },
  { pattern: /\bLumenAI\b/g, label: 'LumenAI' },
]

const PUBLISHED_PRESETS = new Set(['minimal', 'basic', 'brave', 'custom'])

// Key pages the documentation guidance requires in both languages.
const KEY_PAIRS: [string, string][] = [
  ['en/index.md', 'zh-cn/index.md'],
  ['en/user-manual/introduction/index.md', 'zh-cn/user-manual/introduction/index.md'],
  ['en/user-manual/introduction/installation.md', 'zh-cn/user-manual/introduction/installation.md'],
  ['en/user-manual/introduction/repositories.md', 'zh-cn/user-manual/introduction/repositories.md'],
  ['en/user-manual/introduction/integrity.md', 'zh-cn/user-manual/introduction/integrity.md'],
  ['en/user-manual/features/index.md', 'zh-cn/user-manual/features/index.md'],
  ['en/user-manual/features/manage.md', 'zh-cn/user-manual/features/manage.md'],
  ['en/user-manual/features/settings.md', 'zh-cn/user-manual/features/settings.md'],
  ['en/user-manual/features/agent.md', 'zh-cn/user-manual/features/agent.md'],
  ['en/user-manual/features/lumen-intelligence.md', 'zh-cn/user-manual/features/lumen-intelligence.md'],
]

const errors: string[] = []

function fail(message: string) {
  errors.push(message)
}

function walk(dir: string): string[] {
  const files: string[] = []
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    if (entry === 'internal') continue
    if (statSync(full).isDirectory()) {
      files.push(...walk(full))
    } else if (entry.endsWith('.md')) {
      files.push(full)
    }
  }
  return files
}

function headingsOf(content: string): Set<string> {
  const slugs = new Set<string>()
  for (const line of content.split('\n')) {
    const match = /^(#{1,6})\s+(.+)$/.exec(line.trim())
    if (!match) continue
    slugs.add(slugify(match[2]))
  }
  return slugs
}

// Matches VitePress / github-slugger behavior for ASCII anchors, which is what
// the docs actually link to. CJK characters are kept.
function slugify(text: string): string {
  return text
    .trim()
    .toLowerCase()
    .replace(/[\p{P}\p{S}]+/gu, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '')
}

function resolveLink(fromFile: string, href: string): string | null {
  if (/^(https?:|mailto:|#)/.test(href)) return null
  const [pathPart, frag] = href.split('#')
  if (!pathPart) return null

  let target: string
  if (pathPart.startsWith('/')) {
    // Site-absolute path: /zh-cn/user-manual/... or /user-manual/... (en).
    const stripped = pathPart.replace(/^\//, '')
    const candidate = stripped.startsWith('zh-cn/') || stripped.startsWith('en/')
      ? stripped
      : join('en', stripped)
    target = resolve(root, candidate)
  } else {
    target = resolve(dirname(fromFile), pathPart)
  }

  if (exists(target) && statSync(target).isDirectory()) target = join(target, 'index.md')
  if (!target.endsWith('.md')) target += '.md'

  if (!exists(target)) return target
  if (!frag) return null

  const headings = headingsOf(readFileSync(target, 'utf8'))
  return headings.has(frag) ? null : `${target}#${frag}`
}

function exists(path: string): boolean {
  try {
    statSync(path)
    return true
  } catch {
    return false
  }
}

function checkFile(file: string) {
  const rel = relative(root, file)
  const content = readFileSync(file, 'utf8')
  const body = content.replace(/^---[\s\S]*?---/, '')

  // 1. Forbidden product names.
  for (const { pattern, label } of FORBIDDEN) {
    if (pattern.test(body)) {
      fail(`${rel}: 禁止出现产品名 “${label}”`)
    }
  }

  // 2. The removed overwrite conflict policy.
  if (/\boverwrite\b/.test(body)) {
    fail(`${rel}: 禁止出现已删除的 overwrite 冲突策略`)
  }

  // 3. Published presets only: a backticked preset-like token on a line that
  //    also mentions preset/预设/方案 must be one of the published presets.
  //    Known artifact names (lumen-* files, LUMEN_* variables) are not presets.
  for (const line of body.split('\n')) {
    if (!/\b(preset|预设|能力方案)\b/i.test(line)) continue
    for (const match of line.matchAll(/`([a-z][a-z0-9-]*)`/g)) {
      const token = match[1]
      if (token.startsWith('lumen-') || token.startsWith('LUMEN_')) continue
      if (!PUBLISHED_PRESETS.has(token)) {
        fail(`${rel}: 出现未发布的预设名 “${token}”`)
      }
    }
  }

  // 4. Internal links resolve to files and fragments.
  for (const match of body.matchAll(/\[[^\]]*\]\(([^)\s]+)\)/g)) {
    const href = match[1]
    if (/^(https?:|mailto:)/.test(href)) continue
    const broken = resolveLink(file, href)
    if (broken) {
      fail(`${rel}: 链接无法解析 → ${broken}`)
    }
  }
}

// Bilingual parity.
for (const [en, zh] of KEY_PAIRS) {
  for (const [path, lang] of [[en, 'en'], [zh, 'zh-cn']] as const) {
    if (!exists(join(root, path))) {
      fail(`关键双语页缺失 (${lang}): ${path}`)
    }
  }
}

for (const file of walk(root)) checkFile(file)

if (errors.length) {
  console.error(`docs-checks: ${errors.length} error(s)`)
  for (const error of errors) console.error(`  ✗ ${error}`)
  process.exit(1)
}

console.log(`docs-checks: OK — ${walk(root).length} pages, ${KEY_PAIRS.length} key bilingual pairs`)
