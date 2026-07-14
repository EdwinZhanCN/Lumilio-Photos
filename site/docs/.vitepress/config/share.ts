import { defineConfig } from 'vitepress'
import timeline from "vitepress-markdown-timeline"
// @ts-ignore
import { loadEnv } from 'vite'
// @ts-ignore
import { groupIconMdPlugin, groupIconVitePlugin } from 'vitepress-plugin-group-icons'
import { withMermaid } from "vitepress-plugin-mermaid";
import mediaManifest from '../media-manifest.json'


const mode = process.env.NODE_ENV || 'development'
const { VITE_BASE_URL = '/' } = loadEnv(mode, process.cwd())
const mediaOrigin = (process.env.DOCS_MEDIA_ORIGIN || 'https://media.docs.lumilio.org').replace(/\/$/, '')

function externalizeMediaUrls(code: string) {
    return code.replace(/(["'(])\/(images|videos)\/[^"')\s]+/g, (match) => {
        const pathStart = match.slice(1)
        const objectKey = mediaManifest[pathStart as keyof typeof mediaManifest]
        return objectKey ? `${match[0]}${mediaOrigin}/${objectKey}` : match
    })
}

export const sharedConfig = withMermaid(defineConfig({
    head: [
        ['link', { rel: 'icon', href: '/favicon.ico' }],
        [
            'link',
            { rel: 'preconnect', href: 'https://fonts.googleapis.com' }
        ],
        [
            'link',
            { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' }
        ],
        [
            'link',
            { href: 'https://fonts.googleapis.com/css2?family=Roboto&display=swap', rel: 'stylesheet' }
        ]
    ],
    rewrites: { // 很重要，
        'en/:rest*': ':rest*'
    },
    metaChunk: true,
    // Agent harness docs are raw markdown written for GitHub (they contain
    // `<...>`/`{...}` that the Vue markdown compiler can't parse). Keep them
    // co-located under site/docs/internal/agent but out of the VitePress build,
    // so they're never compiled, deployed, or searchable. internal/frontend
    // (authored VitePress-safe) is built but kept out of nav/sidebar/search.
    srcExclude: ['internal/agent/**'],
    lang: 'en',
    title: "Lumilio Photos",
    description: "Next-Gen Lightweight High-performance Media Manage Web App",
    appearance: true, // 主题模式，默认浅色且开启切换
    base: VITE_BASE_URL,
    lastUpdated: true, // 上次更新
    vite: {
        build: {
            chunkSizeWarningLimit: 1600
        },
        plugins: [
            {
                name: 'lumilio-r2-docs-media',
                enforce: 'pre',
                transform(code, id) {
                    const sourceId = id.split('?', 1)[0]
                    if (!sourceId.includes('/docs/')) return null

                    const transformed = externalizeMediaUrls(code)
                    return transformed === code ? null : { code: transformed, map: null }
                },
            },
            groupIconVitePlugin()
        ],
        server: {
            port: 18089
        }
    },
    markdown: { // markdown 配置
        math: true,
        lineNumbers: true, // 行号显示
        image: {
            // 开启图片懒加载
            lazyLoading: true
        },
        config: (md) => {
            md.use(timeline)
            md.use(groupIconMdPlugin)
        }
    },
    themeConfig: {
        logo: '/logo.png',
        socialLinks: [
            { icon: 'github', link: 'https://github.com/EdwinZhanCN/Lumilio-Photos' }
        ],
        langMenuLabel: "Change Language",
    },
    mermaid: {
        
    },
}))
