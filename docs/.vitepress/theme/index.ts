// https://vitepress.dev/guide/custom-theme
import type { Theme } from "vitepress";
import DefaultTheme from "vitepress/theme";
import "./style.css";
import Layout from "./Layout.vue";
import "virtual:group-icons.css";

export default {
    extends: DefaultTheme,
    Layout,
    enhanceApp({ app, router, siteData }) {},
} satisfies Theme;
