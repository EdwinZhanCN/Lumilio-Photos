import { DefaultTheme } from "vitepress";
import typedocSidebar from "./typedoc-sidebar.json";

export const enSidebar: DefaultTheme.Sidebar = {
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
  "/developer-documentation/": [
    {
      text: "Tech Stack",
      items: [
        {
          text: "Overview",
          link: "/developer-documentation/techstack-overview",
        },
        { text: "Frontend", link: "/developer-documentation/frontend" },
        { text: "Backend", link: "/developer-documentation/backend" },
      ],
    },
    {
      text: "Business Diagram",
      items: [
        {
          text: "Upload[Frontend]",
          link: "/developer-documentation/business-diagram/upload-frontend",
        },
        {
          text: "Upload[Backend]",
          link: "/developer-documentation/business-diagram/upload-backend",
        },
      ],
    },
    {
      text: "Backend",
      items: [
        {
          text: "Processors",
          collapsed: true,
          items: [
            {
              text: "AssetProcessor",
              link: "/developer-documentation/backend/processors/asset-processor",
            },
          ],
        },
      ],
    },
    {
      text: "TypeDoc",
      link: "/docs/",
    },
  ],
  "/docs/": [
    {
      text: "TypeDoc",
      items: typedocSidebar,
    },
  ],
};
