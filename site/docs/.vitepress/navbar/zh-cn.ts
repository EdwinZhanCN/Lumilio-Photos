import { DefaultTheme } from 'vitepress'

// 中文
export const zhcnNav: DefaultTheme.NavItem[] = [
    { text: '主页', link: '/zh-cn' },
    { text: '接口文档', target: '_blank', link: '/redoc-static.html' },
    {
        text: '用户手册',
        items: [
            {
                text: '介绍',
                items: [
                    { text: '概览', link: '/zh-cn/user-manual/introduction/' },
                ],
            },
            {
                text: '功能',
                items: [
                    { text: '概览', link: '/zh-cn/user-manual/features/' },
                ],
            },
        ],
    },
]
