import { defineConfig } from 'vitepress'
import { sharedConfig } from './config/share'
import { zhcnConfig } from './config/zh-cn'
import { enConfig } from './config/en'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  ...sharedConfig,
  locales: {
    root: {
      label: 'English',
      lang: 'en',
      ...enConfig,
    },
    'zh-cn': {
      label: '简体中文',
      lang: 'zh-Hans', // 可选，将作为 `lang` 属性添加到 `html` 标签中
      link: '/zh-cn/', // 默认 /fr/ -- 显示在导航栏翻译菜单上，可以是外部的
      ...zhcnConfig,
    }
  },
})