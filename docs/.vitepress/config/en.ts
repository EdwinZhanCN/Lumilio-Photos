import { enNav } from '../navbar'
import { enSidebar } from '../sidebar'
import dayjs from 'dayjs'
import type { DefaultTheme, LocaleSpecificConfig } from 'vitepress'

export const enConfig: LocaleSpecificConfig<DefaultTheme.Config> = {
    themeConfig: { // 主题设置
        nav: enNav,
        sidebar: enSidebar, // 侧边栏
        footer: { // 页脚
            copyright: `Copyright © ${dayjs().format("YYYY")} EdwinZhanCN`
        },
        outline: { // 大纲显示 1-6 级标题
            level: [1, 6],
        }
    }
}