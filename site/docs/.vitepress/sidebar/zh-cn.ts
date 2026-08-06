import { DefaultTheme } from 'vitepress'
export const zhcnSidebar: DefaultTheme.Sidebar = {
    '/zh-cn/user-manual/': [
        {
            text: '开始使用',
            collapsed: false,
            items: [
                { text: '安装', link: '/zh-cn/user-manual/introduction/installation' },
                { text: '第一次设置', link: '/zh-cn/user-manual/introduction/first-use' },
                { text: '导入第一批媒体', link: '/zh-cn/user-manual/features/manage' },
                { text: '创建并下载第一份数据库备份', link: '/zh-cn/user-manual/introduction/integrity' },
            ]
        },
        {
            text: '使用流明集（Lumilio Photos）',
            collapsed: false,
            items: [
                { text: '浏览与搜索', link: '/zh-cn/user-manual/features/assets' },
                { text: '相册与 Collection', link: '/zh-cn/user-manual/features/collections' },
                { text: '人物与事件', link: '/zh-cn/user-manual/features/people' },
                { text: 'Lumilio Agent', link: '/zh-cn/user-manual/features/agent' },
            ]
        },
        {
            text: 'AI 与计算',
            collapsed: false,
            items: [
                { text: 'Lumilio Agent：provider、隐私与权限', link: '/zh-cn/user-manual/features/agent' },
                { text: 'Lumen Intelligence', link: '/zh-cn/user-manual/features/lumen-intelligence' },
            ]
        },
        {
            text: '安全运维',
            collapsed: false,
            items: [
                { text: 'Repository 与原件策略', link: '/zh-cn/user-manual/introduction/repositories' },
                { text: '备份与恢复', link: '/zh-cn/user-manual/introduction/integrity' },
                { text: '升级', link: '/zh-cn/user-manual/introduction/upgrade' },
                { text: '诊断与日志', link: '/zh-cn/user-manual/features/monitor' },
            ]
        },
        {
            text: '高级与参考',
            collapsed: false,
            items: [
                { text: '认识流明集（Lumilio Photos）', link: '/zh-cn/user-manual/introduction/' },
                { text: '使用指南', link: '/zh-cn/user-manual/features/' },
                { text: '首页与统计', link: '/zh-cn/user-manual/features/home' },
                { text: '工作室', link: '/zh-cn/user-manual/features/studio' },
                { text: '分享链接', link: '/zh-cn/user-manual/features/sharing' },
                { text: '重复项、收藏与回收站', link: '/zh-cn/user-manual/features/utilities' },
                { text: '账户、用户与偏好', link: '/zh-cn/user-manual/features/settings' },
                { text: 'HTTPS 与通行密钥', link: '/zh-cn/user-manual/help/https' },
                { text: '恢复管理员访问', link: '/zh-cn/user-manual/introduction/break-glass' },
                { text: '测试版与可选功能', link: '/zh-cn/user-manual/introduction/experimental' },
            ]
        },
    ],
}
