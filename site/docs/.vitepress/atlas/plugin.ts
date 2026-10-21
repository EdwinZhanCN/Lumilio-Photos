// Serves the Atlas model (written by `task atlas:data` to .local/atlas) as
// virtual modules and reloads the open page when the files change. A Vite
// plugin rather than a VitePress data loader, because loaders do not watch
// files outside the site root.
import { existsSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import type { Plugin } from 'vite'

const modules: Record<string, { file: string; empty: string }> = {
  'virtual:lumilio-atlas': { file: 'atlas.json', empty: 'null' },
  'virtual:lumilio-atlas-symbols': { file: 'atlas-symbols.json', empty: '[]' },
}

export function atlasData(siteRoot: string, enabled: boolean): Plugin {
  const dir = resolve(siteRoot, '../.local/atlas')
  const path = (id: string) => resolve(dir, modules[id].file)
  return {
    name: 'lumilio-atlas-data',
    resolveId(id) {
      return id in modules ? `\0${id}` : undefined
    },
    load(id) {
      const name = id.startsWith('\0') ? id.slice(1) : ''
      if (!(name in modules)) return undefined
      const file = path(name)
      const body = enabled && existsSync(file) ? JSON.stringify(readFileSync(file, 'utf8')) : null
      return body ? `export default JSON.parse(${body})` : `export default ${modules[name].empty}`
    },
    configureServer(server) {
      if (!enabled) return
      const files = Object.keys(modules).map(path)
      server.watcher.add(files)
      server.watcher.on('change', (file) => {
        if (!files.includes(file)) return
        for (const name of Object.keys(modules)) {
          const module = server.moduleGraph.getModuleById(`\0${name}`)
          if (module) server.moduleGraph.invalidateModule(module)
        }
        server.ws.send({ type: 'full-reload' })
      })
    },
  }
}
