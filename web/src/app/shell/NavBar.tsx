import { Menu } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { MessageCenter } from "@/features/notifications";
import { NavbarUploadQueue } from "@/features/upload";
import { Breadcrumbs } from "@/components/breadcrumbs";

/**
 * App top bar: current context + in-progress activity.
 * Brand, account, and theme live in the sidebar ("home and where to go").
 */
function NavBar() {
  const { t } = useI18n();

  return (
    <div className="navbar min-h-12 bg-base-100 px-2 sm:px-4 py-1 border-b border-base-300/60">
      <div className="flex shrink-0 items-center">
        <label
          htmlFor="app-drawer"
          aria-label={t("sidebar.openMenu", { defaultValue: "Open menu" })}
          className="btn btn-square btn-ghost btn-sm lg:hidden"
        >
          <Menu className="size-5" />
        </label>
      </div>

      <div className="min-w-0 flex-1">
        <Breadcrumbs />
      </div>

      <div className="flex shrink-0 items-center gap-1 sm:gap-2">
        <MessageCenter />
        <NavbarUploadQueue />
      </div>
    </div>
  );
}

export default NavBar;
