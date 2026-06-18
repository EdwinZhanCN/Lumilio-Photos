import { DefaultTheme } from "vitepress";

// 英文导航
export const enNav: DefaultTheme.NavItem[] = [
  { text: "Home", link: "/" },
  { text: "API Documentation", target: "_blank", link: "/redoc-static.html" },
  {
    text: "User Manual",
    items: [
      {
        text: "Introduction",
        items: [{ text: "Overview", link: "/user-manual/introduction/" }],
      },
      {
        text: "Features",
        items: [{ text: "Overview", link: "/user-manual/features/" }],
      },
    ],
  },
];
