import { DefaultTheme } from 'vitepress'
export const zhcnSidebar: DefaultTheme.Sidebar = {
    '/zh-cn/user-manual/': [
        {
            text: '开始使用',
            collapsed: false,
            items: [
                { text: '认识 Lumilio Photos', link: '/zh-cn/user-manual/introduction/' },
                { text: '安装 Lumilio Photos', link: '/zh-cn/user-manual/introduction/installation' },
                { text: '完成首次设置', link: '/zh-cn/user-manual/introduction/first-use' },
                { text: '理解资源库与原始文件', link: '/zh-cn/user-manual/introduction/repositories' },
            ]
        },
        {
            text: '整理与查看',
            collapsed: false,
            items: [
                { text: '把照片加入资源库', link: '/zh-cn/user-manual/features/manage' },
                { text: '浏览、筛选与批量操作', link: '/zh-cn/user-manual/features/assets' },
                { text: '相册与合集', link: '/zh-cn/user-manual/features/collections' },
                { text: '人物、地点与事件', link: '/zh-cn/user-manual/features/people' },
                { text: '重复项、收藏与回收站', link: '/zh-cn/user-manual/features/utilities' },
            ]
        },
        {
            text: '创作与分享',
            collapsed: false,
            items: [
                { text: '在工作室编辑照片', link: '/zh-cn/user-manual/features/studio' },
                { text: '创建与管理分享链接', link: '/zh-cn/user-manual/features/sharing' },
                { text: '可选的 Lumen 与 Agent', link: '/zh-cn/user-manual/features/lumen-ai' },
            ]
        },
        {
            text: '维护与排障',
            collapsed: false,
            items: [
                { text: '账户、用户与偏好', link: '/zh-cn/user-manual/features/settings' },
                { text: '检查任务与服务状态', link: '/zh-cn/user-manual/features/monitor' },
                { text: '备份与数据完整性', link: '/zh-cn/user-manual/introduction/integrity' },
                { text: '恢复管理员访问', link: '/zh-cn/user-manual/introduction/break-glass' },
            ]
        },
        {
            text: '更多帮助与支持',
            collapsed: false,
            items: [
                { text: 'HTTPS 与通行密钥', link: '/zh-cn/user-manual/help/https' },
                { text: '存储细节', link: '/zh-cn/user-manual/help/storage-details' },
            ]
        },
    ],
}
