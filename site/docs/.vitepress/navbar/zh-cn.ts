import { DefaultTheme } from 'vitepress'

// 中文
export const zhcnNav: DefaultTheme.NavItem[] = [
    { text: '主页', link: '/zh-cn' },
    { text: '接口文档', target: '_blank', link: '/redoc-static.html' },
    {
        text: '用户手册',
        link: '/zh-cn/user-manual/introduction/',
    },
]
