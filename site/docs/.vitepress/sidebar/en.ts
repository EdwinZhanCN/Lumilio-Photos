import { DefaultTheme } from "vitepress";

export const enSidebar: DefaultTheme.Sidebar = {
  "/user-manual/": [
    {
      text: "Getting Started",
      collapsed: false,
      items: [
        { text: "Installation", link: "/user-manual/introduction/installation" },
        { text: "First-time setup", link: "/user-manual/introduction/first-use" },
        { text: "Import your first media", link: "/user-manual/features/manage" },
        { text: "Create and download your first database backup", link: "/user-manual/introduction/integrity" },
      ],
    },
    {
      text: "Using Lumilio Photos",
      collapsed: false,
      items: [
        { text: "Browse & Search", link: "/user-manual/features/assets" },
        { text: "Albums & Collections", link: "/user-manual/features/collections" },
        { text: "People & Events", link: "/user-manual/features/people" },
        { text: "Lumilio Agent", link: "/user-manual/features/agent" },
      ],
    },
    {
      text: "AI & Compute",
      collapsed: false,
      items: [
        { text: "Lumilio Agent: provider, privacy & permissions", link: "/user-manual/features/agent" },
        { text: "Lumen Intelligence", link: "/user-manual/features/lumen-intelligence" },
      ],
    },
    {
      text: "Security & Operations",
      collapsed: false,
      items: [
        { text: "Repositories & original-file policy", link: "/user-manual/introduction/repositories" },
        { text: "Backup & Restore", link: "/user-manual/introduction/integrity" },
        { text: "Upgrade", link: "/user-manual/introduction/upgrade" },
        { text: "Diagnostics & Logs", link: "/user-manual/features/monitor" },
      ],
    },
    {
      text: "Advanced & Reference",
      collapsed: false,
      items: [
        { text: "Overview", link: "/user-manual/introduction/" },
        { text: "Features", link: "/user-manual/features/" },
        { text: "Settings", link: "/user-manual/features/settings" },
        { text: "Recover administrator access", link: "/user-manual/introduction/break-glass" },
      ],
    },
  ],
};
