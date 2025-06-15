import { DefaultTheme } from "vitepress";

// 英文导航
export const enNav: DefaultTheme.NavItem[] = [
    { text: "Home", link: "/" },
    { text: "API Documentation", target: "_blank", link: "/redoc-static.html" },
    { text: "User Manual", link: "/user-manual/user-manual-overview" },
    { text: "Tech Stack", link: "/tech-stack/techstack-overview" },
    { text: "Developer Documentation", link: "/docs/modules" },
];
