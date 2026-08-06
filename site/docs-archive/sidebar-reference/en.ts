import { DefaultTheme } from "vitepress";

export const enSidebar: DefaultTheme.Sidebar = {
  "/user-manual/": [
    {
      text: "Getting Started",
      collapsed: false,
      items: [
        { text: "Overview", link: "/user-manual/introduction/" },
        { text: "Features", link: "/user-manual/features/" },
        { text: "Installation", link: "/user-manual/introduction/installation" },
        { text: "Add Media to a Repository", link: "/user-manual/features/manage" },
      ],
    },
    {
      text: "Using Lumilio Photos",
      collapsed: false,
      items: [
        { text: "Home", link: "/user-manual/features/home" },
        { text: "Browse, Filter & Batch", link: "/user-manual/features/assets" },
        { text: "Collections", link: "/user-manual/features/collections" },
        { text: "Albums", link: "/user-manual/features/albums" },
        { text: "Utilities", link: "/user-manual/features/utilities" },
        { text: "Studio", link: "/user-manual/features/studio" },
        { text: "Lumilio Agent", link: "/user-manual/features/agent" },
      ],
    },
    {
      text: "AI & Compute",
      collapsed: false,
      items: [
        { text: "Lumen Intelligence", link: "/user-manual/features/lumen-intelligence" },
        { text: "Lumen Intelligence in depth", link: "/user-manual/help/lumen-intelligence-details" },
        { text: "Lumilio Agent in depth", link: "/user-manual/help/agent-details" },
        { text: "Lumilio Agent: configuration & privacy", link: "/user-manual/features/agent" },
        { text: "AI settings (Settings → AI)", link: "/user-manual/features/settings" },
      ],
    },
    {
      text: "Security & Operations",
      collapsed: false,
      items: [
        { text: "Storage Locations and Repositories", link: "/user-manual/introduction/repositories" },
        { text: "Account, Users & Preferences", link: "/user-manual/features/settings" },
        { text: "Backup and Data Integrity", link: "/user-manual/introduction/integrity" },
        { text: "Server Monitor", link: "/user-manual/features/monitor" },
        { text: "Recover administrator access", link: "/user-manual/introduction/break-glass" },
      ],
    },
  ],
};
