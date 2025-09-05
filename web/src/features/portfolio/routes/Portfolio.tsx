import PageHeader from "@/components/PageHeader";
import { BookOpenIcon } from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";

export function Portfolio() {
  const { t } = useI18n();
  return (
    <PageHeader
      title={t("routes.portfolio")}
      icon={<BookOpenIcon className="w-6 h-6 text-primary" />}
    />
  );
}
