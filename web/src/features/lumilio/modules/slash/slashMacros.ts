/** Quick action definitions, i18n-driven.
 *
 * Quick actions are triggerable from the `/` slash menu or the input-bar
 * Plus button. Selecting one sends an open-ended task description plus a
 * `mode` that constrains the agent's visible tool subset (progressive
 * disclosure). Empty-state quick-asks serve the same function in chip form. */

import { useI18n } from "@/lib/i18n";

export interface SlashMacro {
  id: string;
  label: string;
  description: string;
  template: string;
  mode: string;
}

export function useSlashMacros(): SlashMacro[] {
  const { t } = useI18n();
  return [
    {
      id: "review",
      mode: "review",
      label: t("lumilio.quickActions.review.label", "Review"),
      description: t(
        "lumilio.quickActions.review.description",
        "Review a period of your photography journey",
      ),
      template: t(
        "lumilio.quickActions.review.template",
        "Help me review my photography journey this year",
      ),
    },
    {
      id: "organize",
      mode: "organize",
      label: t("lumilio.quickActions.organize.label", "Organize"),
      description: t(
        "lumilio.quickActions.organize.description",
        "Group and tag photos, create albums",
      ),
      template: t("lumilio.quickActions.organize.template", "Help me organize my recent photos"),
    },
    {
      id: "analyze",
      mode: "analyze",
      label: t("lumilio.quickActions.analyze.label", "Analyze"),
      description: t(
        "lumilio.quickActions.analyze.description",
        "Discover shooting habits and trends",
      ),
      template: t("lumilio.quickActions.analyze.template", "Analyze my shooting habits and trends"),
    },
    {
      id: "curate",
      mode: "curate",
      label: t("lumilio.quickActions.curate.label", "Curate"),
      description: t("lumilio.quickActions.curate.description", "Pick the best photos"),
      template: t(
        "lumilio.quickActions.curate.template",
        "Help me curate the best photos from recent uploads",
      ),
    },
  ];
}
