import { Bell, CheckCheck, Trash2 } from "lucide-react";
import { useGlobal } from "@/contexts/GlobalContext";
import { useI18n } from "@/lib/i18n.tsx";

function formatRelativeTime(createdAt: number, language: string) {
  const delta = Math.max(0, Math.floor((Date.now() - createdAt) / 1000));
  if (delta < 60)
    return new Intl.RelativeTimeFormat(language).format(-delta, "second");
  if (delta < 3600) {
    const minutes = Math.floor(delta / 60);
    return new Intl.RelativeTimeFormat(language).format(-minutes, "minute");
  }
  if (delta < 86400) {
    const hours = Math.floor(delta / 3600);
    return new Intl.RelativeTimeFormat(language).format(-hours, "hour");
  }
  const days = Math.floor(delta / 86400);
  return new Intl.RelativeTimeFormat(language).format(-days, "day");
}

export default function MessageCenter() {
  const { t, i18n } = useI18n();
  const {
    notifications,
    markNotificationRead,
    markAllNotificationsRead,
    clearNotifications,
  } = useGlobal();

  const unreadCount =
    notifications.filter((item) => !item.read).length > 0
      ? notifications.filter((item) => !item.read).length
      : 0;
  const recent = notifications.slice(0, 20);
  const locale = i18n.resolvedLanguage || i18n.language || "en";

  return (
    <div className="dropdown dropdown-end">
      <button
        type="button"
        tabIndex={0}
        className="btn btn-ghost gap-2 rounded-full"
      >
        <Bell className="w-5 h-5" />
        <span className="badge badge-secondary badge-sm">
          {unreadCount > 99 ? "99+" : unreadCount}
        </span>
      </button>
      <div
        tabIndex={0}
        className="dropdown-content z-30 mt-2 w-96 max-w-[calc(100vw-2rem)] rounded-2xl border border-base-300 bg-base-100 shadow-xl"
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-base-300">
          <h3 className="font-semibold">{t("navbar.notifications.title")}</h3>
          <div className="flex items-center gap-1">
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={markAllNotificationsRead}
              title={t("navbar.notifications.markAllRead")}
            >
              <CheckCheck className="w-3 h-3" />
            </button>
            <button
              type="button"
              className="btn btn-ghost btn-xs"
              onClick={clearNotifications}
              title={t("navbar.notifications.clear")}
            >
              <Trash2 className="w-3 h-3" />
            </button>
          </div>
        </div>

        {recent.length === 0 ? (
          <div className="px-4 py-8 text-sm text-base-content/60 text-center">
            {t("navbar.notifications.empty")}
          </div>
        ) : (
          <ul className="max-h-80 overflow-y-auto divide-y divide-base-300/50">
            {recent.map((item) => (
              <li
                key={item.id}
                className={`px-4 py-3 ${item.read ? "opacity-70" : ""}`}
                onMouseEnter={() => markNotificationRead(item.id)}
              >
                <div className="flex items-start justify-between gap-3">
                  <p className="text-sm leading-5">{item.message}</p>
                  <span className="text-[11px] text-base-content/50 whitespace-nowrap">
                    {formatRelativeTime(item.createdAt, locale)}
                  </span>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
