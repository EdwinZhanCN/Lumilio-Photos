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
        ['link', { rel: 'icon', href: '/favicon.ico' }]
    ],
    rewrites: { // 很重要，
        'en/:rest*': ':rest*'
    },
    metaChunk: true,
    // Internal engineering docs are written for repository readers and may
    // contain Markdown that Vue's compiler treats as template syntax. Keep the
    // whole internal tree out of the public build, deployment, and search.
    srcExclude: ['internal/**'],
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
