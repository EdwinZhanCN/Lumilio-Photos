// https://vitepress.dev/guide/custom-theme
import type { Theme } from "vitepress";
import DefaultTheme from "vitepress/theme";
import "./style.css";
import Layout from "./Layout.vue";
import "virtual:group-icons.css";
import { Search, Plus } from "@lucide/vue";
import DocPath from "../components/DocPath.vue";
import RepositoryTree from "../components/RepositoryTree.vue";
import ImportScenarios from "../components/ImportScenarios.vue";

export default {
    extends: DefaultTheme,
    Layout,
    enhanceApp({ app, router, siteData }) {
        app.component("Search", Search);
        app.component("Plus", Plus);
        app.component("DocPath", DocPath);
        app.component("RepositoryTree", RepositoryTree);
        app.component("ImportScenarios", ImportScenarios);
    },
} satisfies Theme;
