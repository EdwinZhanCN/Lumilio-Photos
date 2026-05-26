import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { BookOpenIcon } from "lucide-react";

export function Portfolio() {
  const { t } = useI18n();
  return (
    <PageHeader
      title={t("routes.portfolio")}
      icon={<BookOpenIcon className="w-6 h-6 text-primary" />}
    />
  );
}
