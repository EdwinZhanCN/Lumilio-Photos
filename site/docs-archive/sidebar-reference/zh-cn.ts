import { DefaultTheme } from 'vitepress'
export const zhcnSidebar: DefaultTheme.Sidebar = {
    '/zh-cn/user-manual/': [
        {
            text: '开始使用',
            collapsed: false,
            items: [
                { text: '认识 Lumilio Photos', link: '/zh-cn/user-manual/introduction/' },
                { text: '使用指南', link: '/zh-cn/user-manual/features/' },
                { text: '首页与统计', link: '/zh-cn/user-manual/features/home' },
                { text: '安装 Lumilio Photos', link: '/zh-cn/user-manual/introduction/installation' },
                { text: '完成首次设置', link: '/zh-cn/user-manual/introduction/first-use' },
                { text: '把媒体加入资源库', link: '/zh-cn/user-manual/features/manage' },
            ]
        },
        {
            text: '使用流明集（Lumilio Photos）',
            collapsed: false,
            items: [
                { text: '浏览、筛选与批量操作', link: '/zh-cn/user-manual/features/assets' },
                { text: '相册与合集', link: '/zh-cn/user-manual/features/collections' },
                { text: '人物、地点与事件', link: '/zh-cn/user-manual/features/people' },
                { text: '重复项、收藏与回收站', link: '/zh-cn/user-manual/features/utilities' },
                { text: '在工作室编辑媒体', link: '/zh-cn/user-manual/features/studio' },
                { text: '创建与管理分享链接', link: '/zh-cn/user-manual/features/sharing' },
                { text: 'Lumilio Agent', link: '/zh-cn/user-manual/features/agent' },
            ]
        },
        {
            text: 'AI 与计算',
            collapsed: false,
            items: [
                { text: 'Lumen Intelligence', link: '/zh-cn/user-manual/features/lumen-intelligence' },
                { text: 'Lumen Intelligence 进阶（网络与排障）', link: '/zh-cn/user-manual/help/lumen-intelligence-details' },
                { text: 'Lumilio Agent 进阶（引用、结果集与确认）', link: '/zh-cn/user-manual/help/agent-details' },
                { text: 'Lumilio Agent：配置、提供方与隐私', link: '/zh-cn/user-manual/features/agent' },
                { text: 'AI 设置（设置 → AI）', link: '/zh-cn/user-manual/features/settings' },
            ]
        },
        {
            text: '安全运维',
            collapsed: false,
            items: [
                { text: '理解资源库与原始文件', link: '/zh-cn/user-manual/introduction/repositories' },
                { text: '账户、用户与偏好', link: '/zh-cn/user-manual/features/settings' },
                { text: '备份与数据完整性', link: '/zh-cn/user-manual/introduction/integrity' },
                { text: '检查任务与服务状态', link: '/zh-cn/user-manual/features/monitor' },
                { text: '恢复管理员访问', link: '/zh-cn/user-manual/introduction/break-glass' },
            ]
        },
        {
            text: '高级与参考',
            collapsed: false,
            items: [
                { text: 'HTTPS 与通行密钥', link: '/zh-cn/user-manual/help/https' },
                { text: '存储细节', link: '/zh-cn/user-manual/help/storage-details' },
                { text: '导入与扫描进阶', link: '/zh-cn/user-manual/help/ingest-details' },
                { text: '浏览与搜索进阶', link: '/zh-cn/user-manual/help/browse-details' },
                { text: '相册与合集进阶', link: '/zh-cn/user-manual/help/collections-details' },
                { text: '云导入进阶', link: '/zh-cn/user-manual/help/cloud-details' },
                { text: '分享链接进阶', link: '/zh-cn/user-manual/help/sharing-details' },
                { text: '工作室进阶', link: '/zh-cn/user-manual/help/studio-details' },
                { text: '工具进阶', link: '/zh-cn/user-manual/help/utilities-details' },
                { text: '设置与状态进阶', link: '/zh-cn/user-manual/help/maintenance-details' },
                { text: '测试版与可选功能', link: '/zh-cn/user-manual/introduction/experimental' },
            ]
        },
    ],
}
