import { defineConfig } from 'vitepress'
import timeline from "vitepress-markdown-timeline"
import { loadEnv } from 'vite'
import { groupIconMdPlugin, groupIconVitePlugin } from 'vitepress-plugin-group-icons'
import { withMermaid } from "vitepress-plugin-mermaid";


const mode = process.env.NODE_ENV || 'development'
const { VITE_BASE_URL } = loadEnv(mode, process.cwd())

console.log('Mode:', process.env.NODE_ENV)
console.log('VITE_BASE_URL:', VITE_BASE_URL)

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