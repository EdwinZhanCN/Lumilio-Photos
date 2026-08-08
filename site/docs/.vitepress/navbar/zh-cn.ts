import { DefaultTheme } from 'vitepress'

// 中文导航。中文落地页保持独立；这里仅调整主文档入口。
export const zhcnNav: DefaultTheme.NavItem[] = [
    { text: '主页', link: '/zh-cn/' },
    { text: '文档', link: '/zh-cn/user-manual/' },
    { text: '开始使用', link: '/zh-cn/user-manual/getting-started/' },
    { text: '使用流明集', link: '/zh-cn/user-manual/use/' },
    { text: '解决问题', link: '/zh-cn/user-manual/troubleshooting/' },
    { text: '管理与安全', link: '/zh-cn/user-manual/admin/' },
    {
        text: '更多',
        items: [
            { text: '了解原理', link: '/zh-cn/user-manual/concepts/' },
            { text: '参考', link: '/zh-cn/user-manual/reference/' },
            { text: '开发者', link: '/zh-cn/user-manual/developer/' },
        ],
    },
]
