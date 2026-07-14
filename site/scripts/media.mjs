import { createHash } from 'node:crypto'
import { access, mkdir, readFile, readdir, writeFile } from 'node:fs/promises'
import { constants } from 'node:fs'
import { dirname, extname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { spawn } from 'node:child_process'

const siteRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const manifestPath = join(siteRoot, 'docs/.vitepress/media-manifest.json')
const bucket = process.env.DOCS_MEDIA_BUCKET || 'lumilio-docs-media'
const origin = (process.env.DOCS_MEDIA_ORIGIN || 'https://media.docs.lumilio.org').replace(/\/$/, '')
const sourceArgumentIndex = process.argv.indexOf('--source')
if (sourceArgumentIndex !== -1 && !process.argv[sourceArgumentIndex + 1]) {
  throw new Error('--source requires a directory')
}
const sourceRoot = resolve(siteRoot, sourceArgumentIndex === -1 ? 'media' : process.argv[sourceArgumentIndex + 1])
const dryRun = process.argv.includes('--dry-run')
const command = process.argv[2]

const contentTypes = new Map([
  ['.avif', 'image/avif'],
  ['.gif', 'image/gif'],
  ['.jpeg', 'image/jpeg'],
  ['.jpg', 'image/jpeg'],
  ['.png', 'image/png'],
  ['.svg', 'image/svg+xml'],
  ['.webm', 'video/webm'],
  ['.webp', 'image/webp'],
])

function usage() {
  console.error('Usage: node scripts/media.mjs <manifest|sync|verify> [--source <directory>] [--dry-run]')
  process.exitCode = 1
}

async function exists(path) {
  try {
    await access(path, constants.R_OK)
    return true
  } catch {
    return false
  }
}

async function walk(directory) {
  const entries = await readdir(directory, { withFileTypes: true })
  const files = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return walk(path)
    return entry.isFile() ? [path] : []
  }))
  return files.flat()
}

async function sha256(path) {
  return createHash('sha256').update(await readFile(path)).digest('hex')
}

async function mediaFiles() {
  const files = []
  for (const directory of ['images', 'videos']) {
    const path = join(sourceRoot, directory)
    if (await exists(path)) files.push(...await walk(path))
  }
  return files.filter((file) => contentTypes.has(extname(file).toLowerCase())).sort()
}

async function createManifest() {
  const files = await mediaFiles()
  if (files.length === 0) throw new Error(`No media found below ${sourceRoot}/images or ${sourceRoot}/videos`)

  const manifest = {}
  for (const file of files) {
    const logicalPath = `/${relative(sourceRoot, file).split('\\').join('/')}`
    const digest = await sha256(file)
    manifest[logicalPath] = `sha256/${digest}/${logicalPath.slice(1)}`
  }

  await mkdir(dirname(manifestPath), { recursive: true })
  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`)
  console.log(`Wrote ${Object.keys(manifest).length} media entries to ${relative(siteRoot, manifestPath)}`)
  return manifest
}

async function loadManifest() {
  if (!await exists(manifestPath)) {
    throw new Error(`Missing ${relative(siteRoot, manifestPath)}; run media:manifest first`)
  }
  return JSON.parse(await readFile(manifestPath, 'utf8'))
}

function runWrangler(args) {
  return new Promise((resolveRun, rejectRun) => {
    const child = spawn('wrangler', args, { cwd: siteRoot, stdio: 'inherit' })
    child.once('error', rejectRun)
    child.once('exit', (code) => code === 0 ? resolveRun() : rejectRun(new Error(`wrangler exited with status ${code}`)))
  })
}

async function sync() {
  const manifest = await loadManifest()
  const files = await mediaFiles()
  if (files.length === 0) throw new Error(`No media found below ${sourceRoot}/images or ${sourceRoot}/videos`)

  for (const file of files) {
    const logicalPath = `/${relative(sourceRoot, file).split('\\').join('/')}`
    const objectKey = manifest[logicalPath]
    if (!objectKey) throw new Error(`${logicalPath} is not in the manifest; run media:manifest again`)
    if (!objectKey.includes(await sha256(file))) throw new Error(`${logicalPath} no longer matches its manifest hash; run media:manifest again`)

    const contentType = contentTypes.get(extname(file).toLowerCase())
    if (!contentType) throw new Error(`Unsupported media type: ${file}`)
    const args = ['r2', 'object', 'put', `${bucket}/${objectKey}`, '--remote', '--file', file, '--content-type', contentType, '--cache-control', 'public, max-age=31536000, immutable']
    console.log(`${dryRun ? 'Would upload' : 'Uploading'} ${logicalPath} -> ${objectKey}`)
    if (!dryRun) await runWrangler(args)
  }
}

async function verify() {
  const manifest = await loadManifest()
  const failures = []
  for (const [logicalPath, objectKey] of Object.entries(manifest)) {
    const response = await fetch(`${origin}/${objectKey}`, { method: 'HEAD' })
    if (!response.ok) failures.push(`${logicalPath}: ${response.status}`)
  }
  if (failures.length > 0) throw new Error(`R2 verification failed:\n${failures.join('\n')}`)
  console.log(`Verified ${Object.keys(manifest).length} R2 media objects at ${origin}`)
}

try {
  if (command === 'manifest') await createManifest()
  else if (command === 'sync') await sync()
  else if (command === 'verify') await verify()
  else usage()
} catch (error) {
  console.error(error instanceof Error ? error.message : error)
  process.exitCode = 1
}
