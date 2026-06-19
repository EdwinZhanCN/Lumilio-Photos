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
                { text: '首页', link: '/zh-cn/user-manual/features/home' },
                { text: '资源库', link: '/zh-cn/user-manual/features/assets' },
                { text: '合集', link: '/zh-cn/user-manual/features/collections' },
                { text: '相册', link: '/zh-cn/user-manual/features/albums' },
                { text: '实用工具', link: '/zh-cn/user-manual/features/utilities' },
                { text: '工作室', link: '/zh-cn/user-manual/features/studio' },
                { text: '管理', link: '/zh-cn/user-manual/features/manage' },
                { text: '设置', link: '/zh-cn/user-manual/features/settings' },
                { text: '服务监控', link: '/zh-cn/user-manual/features/monitor' },
            ]
        },
    ],
}
