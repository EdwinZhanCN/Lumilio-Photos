import { DefaultTheme } from "vitepress";
import typedocSidebar from "./typedoc-sidebar.json";

export const enSidebar: DefaultTheme.Sidebar = {
    "/backend-API/": [
        {
            text: "API Documentation",
            items: [
                { text: "Overview", link: "/backend-API/backend-api-overview" },
                { text: "Assets", link: "/backend-API/assets" },
            ],
        },
    ],
    "/user-manual/": [
        {
            text: "User Manual",
            items: [
                { text: "Overview", link: "/user-manual/user-manual-overview" },
                { text: "Installation", link: "/user-manual/Installation" },
                { text: "Key Features", link: "/user-manual/key-feature" },
                {
                    text: "System Settings",
                    link: "/user-manual/system-setting",
                },
                {
                    text: "Advanced Features",
                    link: "/user-manual/advanced-feature",
                },
                {
                    text: "Troubleshooting",
                    link: "/user-manual/troubleshooting",
                },
            ],
        },
    ],
    "/tech-stack/": [
        {
            text: "Tech Stack",
            items: [
                { text: "Overview", link: "/tech-stack/techstack-overview" },
                { text: "Frontend", link: "/tech-stack/frontend" },
                { text: "Backend", link: "/tech-stack/backend" },
            ],
        },
    ],
    "/docs/": [
        {
            text: "TypeDoc",
            items: typedocSidebar,
        },
    ],
};
