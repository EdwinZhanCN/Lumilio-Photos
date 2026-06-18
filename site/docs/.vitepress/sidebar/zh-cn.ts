import { DefaultTheme } from 'vitepress'
export const zhcnSidebar: DefaultTheme.Sidebar = {
    '/zh-cn/user-manual/': [
        {
            text: '介绍',
            collapsed: false,
            items: [
                { text: '概览', link: '/zh-cn/user-manual/introduction/' },
            ]
        },
        {
            text: '功能',
            collapsed: false,
            items: [
                { text: '概览', link: '/zh-cn/user-manual/features/' },
            ]
        },
    ],
}
