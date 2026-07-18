import { useI18n } from "@/lib/i18n.tsx";
import { BookOpenIcon, ExternalLinkIcon, FileBadgeIcon, ScaleIcon } from "lucide-react";
import type { ReactNode } from "react";
import { SettingsGroup, SettingsRow } from "../../components/SettingsGroup";

const REPOSITORY = "https://github.com/EdwinZhanCN/Lumilio-Photos";

export default function AboutTab() {
  const { t, i18n } = useI18n();
  const termsFile = i18n.language.toLowerCase().startsWith("zh")
    ? "TERMS-OF-USE.zh-CN.txt"
    : "TERMS-OF-USE.en.txt";

  return (
    <div className="space-y-8">
      <SettingsGroup
        title={t("settings.about.legalTitle", "Legal")}
        description={t(
          "settings.about.legalDescription",
          "Review the terms and licenses that apply to this installation.",
        )}
      >
        <LegalRow
          icon={<BookOpenIcon className="size-4" />}
          title={t("settings.about.termsTitle", "Terms of Use")}
          description={t(
            "settings.about.termsDescription",
            "Responsibilities, beta software risks, automated analysis, warranty, and liability.",
          )}
          href={`${REPOSITORY}/blob/main/desktop/licenses/${termsFile}`}
          action={t("settings.about.read", "Read")}
        />
        <LegalRow
          icon={<ScaleIcon className="size-4" />}
          title={t("settings.about.licenseTitle", "Open-source license")}
          description={t(
            "settings.about.licenseDescription",
            "Lumilio Photos is licensed under GNU GPL version 3.",
          )}
          href={`${REPOSITORY}/blob/main/LICENSE`}
          action={t("settings.about.viewLicense", "View license")}
        />
        <LegalRow
          icon={<FileBadgeIcon className="size-4" />}
          title={t("settings.about.noticesTitle", "Third-party software notices")}
          description={t(
            "settings.about.noticesDescription",
            "Attribution and license texts for bundled Go, npm, and native dependencies.",
          )}
          href={`${REPOSITORY}/blob/main/desktop/licenses/THIRD_PARTY_NOTICES.txt`}
          action={t("settings.about.viewNotices", "View notices")}
        />
      </SettingsGroup>

      <SettingsGroup
        title={t("settings.about.projectTitle", "Project")}
        description={t(
          "settings.about.projectDescription",
          "Source code, release history, and issue reporting.",
        )}
      >
        <SettingsRow
          icon={<ExternalLinkIcon className="size-4" />}
          iconColor="bg-base-300 text-base-content"
          label="Lumilio Photos"
          description={REPOSITORY}
          control={
            <a className="btn btn-link btn-sm" href={REPOSITORY} target="_blank" rel="noreferrer">
              {t("settings.about.openRepository", "Open repository")}
              <ExternalLinkIcon className="size-3.5" />
            </a>
          }
        />
      </SettingsGroup>
    </div>
  );
}

function LegalRow({
  icon,
  title,
  description,
  href,
  action,
}: {
  icon: ReactNode;
  title: string;
  description: string;
  href: string;
  action: string;
}) {
  return (
    <SettingsRow
      icon={icon}
      iconColor="bg-base-300 text-base-content"
      label={title}
      description={description}
      control={
        <a className="btn btn-link btn-sm" href={href} target="_blank" rel="noreferrer">
          {action}
          <ExternalLinkIcon className="size-3.5" />
        </a>
      }
    />
  );
}
