import PageHeader from "@/components/PageHeader";
import { Info } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";

export default function Updates() {
  const { t } = useI18n();
  return (
    <PageHeader title={t("routes.updates")} icon={<Info className="w-6 h-6 text-primary" />} />
  );
}
