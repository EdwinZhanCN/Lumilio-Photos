import React from "react";
import type { TFunction } from "i18next";
import { Frame } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { FrameTemplate, TemplateFamily } from "../../../modules/frame/frameTemplate";
import { templateName } from "./templateName";

type TemplateGridProps = {
  templates: readonly FrameTemplate[];
  activeTemplateId: string | null;
  previews: ReadonlyMap<string, string>;
  disabled?: boolean;
  onSelect: (templateId: string) => void;
};

/** Spelled out so the i18n extractor can see the keys; it cannot read a computed one. */
function familyLabel(t: TFunction, family: TemplateFamily): string {
  switch (family) {
    case "bar":
      return t("studio.frame.family.bar", { defaultValue: "Info bar" });
    case "margin":
      return t("studio.frame.family.margin", { defaultValue: "Margin" });
    case "editorial":
      return t("studio.frame.family.editorial", { defaultValue: "Editorial" });
    case "overlay":
      return t("studio.frame.family.overlay", { defaultValue: "On photo" });
    case "dual":
      return t("studio.frame.family.dual", { defaultValue: "Dual mark" });
    case "vertical":
      return t("studio.frame.family.vertical", { defaultValue: "Vertical" });
  }
}

const FAMILY_ORDER: TemplateFamily[] = [
  "bar",
  "margin",
  "editorial",
  "overlay",
  "dual",
  "vertical",
];

/**
 * Templates grouped by family, each showing a rendered preview of this photo.
 *
 * A name alone cannot convey what a preset does, and these presets differ
 * mostly in layout — so the preview is the control, and the label is the
 * caption.
 */
export function TemplateGrid({
  templates,
  activeTemplateId,
  previews,
  disabled = false,
  onSelect,
}: TemplateGridProps): React.JSX.Element {
  const { t } = useI18n();

  const byFamily = new Map<TemplateFamily, FrameTemplate[]>();
  for (const template of templates) {
    const bucket = byFamily.get(template.family);
    if (bucket) bucket.push(template);
    else byFamily.set(template.family, [template]);
  }

  return (
    <div className="space-y-3">
      {FAMILY_ORDER.filter((family) => byFamily.has(family)).map((family) => (
        <div key={family}>
          <h4 className="mb-1.5 px-0.5 text-[10px] font-semibold uppercase tracking-wider text-base-content/40">
            {familyLabel(t, family)}
          </h4>
          <div className="grid grid-cols-2 gap-1.5">
            {byFamily.get(family)!.map((template) => {
              const preview = previews.get(template.id);
              const active = template.id === activeTemplateId;
              return (
                <button
                  key={template.id}
                  type="button"
                  onClick={() => onSelect(template.id)}
                  disabled={disabled}
                  aria-pressed={active}
                  className={`group overflow-hidden rounded-lg border bg-base-100 text-left transition ${
                    active
                      ? "border-primary ring-1 ring-primary/40"
                      : "border-base-300 hover:border-base-content/25"
                  }`}
                >
                  <div className="grid aspect-[4/3] place-items-center overflow-hidden bg-base-200">
                    {preview ? (
                      <img
                        src={preview}
                        alt=""
                        className="h-full w-full object-contain"
                        draggable={false}
                      />
                    ) : (
                      <Frame className="h-4 w-4 text-base-content/25" />
                    )}
                  </div>
                  <div
                    className={`truncate px-1.5 py-1 text-[10px] ${
                      active ? "text-primary" : "text-base-content/60"
                    }`}
                  >
                    {templateName(t, template)}
                  </div>
                </button>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}
