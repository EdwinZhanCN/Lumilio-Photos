import { DefaultTheme } from "vitepress";

export const enSidebar: DefaultTheme.Sidebar = {
  "/user-manual/": [
    {
      text: "Introduction",
      collapsed: false,
      items: [
        { text: "Overview", link: "/user-manual/introduction/" },
        {
          text: "Storage Locations and Repositories",
          link: "/user-manual/introduction/repositories",
        },
        { text: "Installation", link: "/user-manual/introduction/installation" },
        { text: "Recover administrator access", link: "/user-manual/introduction/break-glass" },
      ],
    },
    {
      text: "Features",
      collapsed: false,
      items: [
        { text: "Overview", link: "/user-manual/features/" },
        { text: "Home", link: "/user-manual/features/home" },
        { text: "Media Library", link: "/user-manual/features/assets" },
        { text: "Collections", link: "/user-manual/features/collections" },
        { text: "Albums", link: "/user-manual/features/albums" },
        { text: "Utilities", link: "/user-manual/features/utilities" },
        { text: "Studio", link: "/user-manual/features/studio" },
        { text: "Manage Libraries", link: "/user-manual/features/manage" },
        { text: "Settings", link: "/user-manual/features/settings" },
        { text: "Server Monitor", link: "/user-manual/features/monitor" },
      ],
    },
  ],
};
