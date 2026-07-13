import { DefaultTheme } from 'vitepress'
export const zhcnSidebar: DefaultTheme.Sidebar = {
    '/zh-cn/user-manual/': [
        {
            text: '介绍',
            collapsed: false,
            items: [
                { text: '产品概览', link: '/zh-cn/user-manual/introduction/' },
                { text: '核心概念', link: '/zh-cn/user-manual/introduction/concepts' },
                { text: '媒体与仓库', link: '/zh-cn/user-manual/introduction/repositories' },
                { text: '数据完整性与备份', link: '/zh-cn/user-manual/introduction/integrity' },
                { text: 'AI 与 Lumen', link: '/zh-cn/user-manual/introduction/lumen' },
                { text: '实验性功能与已知风险', link: '/zh-cn/user-manual/introduction/experimental' },
                { text: '安装', link: '/zh-cn/user-manual/introduction/installation' },
                { text: '首次使用', link: '/zh-cn/user-manual/introduction/first-use' },
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
                { text: '人物', link: '/zh-cn/user-manual/features/people' },
                { text: '工具', link: '/zh-cn/user-manual/features/utilities' },
                { text: '云端导入', link: '/zh-cn/user-manual/features/cloud-import' },
                { text: '分享', link: '/zh-cn/user-manual/features/sharing' },
                { text: 'Lumilio Agent', link: '/zh-cn/user-manual/features/lumilio' },
                { text: 'Lumen AI', link: '/zh-cn/user-manual/features/lumen-ai' },
                { text: '工作室', link: '/zh-cn/user-manual/features/studio' },
                { text: '管理', link: '/zh-cn/user-manual/features/manage' },
                { text: '设置', link: '/zh-cn/user-manual/features/settings' },
                { text: '服务监控', link: '/zh-cn/user-manual/features/monitor' },
            ]
        },
    ],
}
