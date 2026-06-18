import { DefaultTheme } from "vitepress";

export const enSidebar: DefaultTheme.Sidebar = {
  "/user-manual/": [
    {
      text: "Introduction",
      collapsed: false,
      items: [{ text: "Overview", link: "/user-manual/introduction/" }],
    },
    {
      text: "Features",
      collapsed: false,
      items: [{ text: "Overview", link: "/user-manual/features/" }],
    },
  ],
};
