import { zhcnNav } from '../navbar'
import { zhcnSidebar } from '../sidebar'
import dayjs from 'dayjs'
import type { DefaultTheme, LocaleSpecificConfig } from 'vitepress'

export const zhcnConfig: LocaleSpecificConfig<DefaultTheme.Config> = {
    themeConfig: { // 主题设置
        returnToTopLabel: '返回顶部', // 更改手机端菜单文字显示
        search: {
            provider: 'local',
            options: {
                locales: {
                    zh: {
                        translations: {
                            button: {
                                buttonText: '搜索',
                                buttonAriaLabel: '搜索',
                            },
                            modal: {
                                displayDetails: '显示详细列表',
                                resetButtonTitle: '重制搜索',
                                backButtonTitle: '关闭搜索',
                                noResultsText: '没有找到相关结果',
                                footer: {
                                    selectText: '选择',
                                    selectKeyAriaLabel: 'enter',
                                    navigateText: '切换',
                                    navigateUpKeyAriaLabel: '上方向键',
                                    navigateDownKeyAriaLabel: '下方向键',
                                    closeText: '关闭',
                                    closeKeyAriaLabel: 'esc',
                                },
                            },
                        },
                    },
                },
            },
        },
        nav: zhcnNav,
        sidebar: zhcnSidebar, // 侧边栏
        docFooter: { // 自定义上下页名
            prev: '上一篇', next: '下一篇'
        },
        darkModeSwitchLabel: '深浅模式', // 手机端深浅模式文字修改
        footer: { // 页脚
            copyright: `Copyright © ${dayjs().format("YYYY")} EdwinZhanCN`
        },
        outline: { // 大纲显示 1-6 级标题
            level: [1, 6],
            label: '目录'
        },
    }
}