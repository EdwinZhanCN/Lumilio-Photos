import { DefaultTheme } from 'vitepress'
export const zhcnSidebar: DefaultTheme.Sidebar = {
    '/zh-cn/backend-API/': [
        {
            text: 'API 文档',
            items: [
                { text: '概览', link: '/zh-cn/backend-API/backend-api-overview' },
                { text: '资源管理', link: '/zh-cn/backend-API/assets' },
            ]
        }
    ],
    '/zh-cn/user-manual/': [
        {
            text: '用户手册',
            items: [
                { text: '概览', link: '/zh-cn/user-manual/user-manual-overview' },
                { text: '安装指南', link: '/zh-cn/user-manual/Installation' },
                { text: '核心功能', link: '/zh-cn/user-manual/key-feature' },
                { text: '系统设置', link: '/zh-cn/user-manual/system-setting' },
                { text: '高级功能', link: '/zh-cn/user-manual/advanced-feature' },
                { text: '故障排除', link: '/zh-cn/user-manual/troubleshooting' },
            ]
        }
    ],
    '/zh-cn/tech-stack/': [
        {
            text: '技术栈',
            items: [
                { text: '概览', link: '/zh-cn/tech-stack/techstack-overview' },
                { text: '前端', link: '/zh-cn/tech-stack/frontend' },
                { text: '后端', link: '/zh-cn/tech-stack/backend' },
            ]
        }
    ],
}