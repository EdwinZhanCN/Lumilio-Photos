import type { TFunction } from "i18next";
import type { FrameTemplate } from "../../../modules/frame/frameTemplate";

/**
 * Translates a template's display name.
 *
 * The keys are spelled out as literal `t()` calls rather than resolved from
 * `template.nameKey`, because the extractor works by static analysis: a
 * computed key is invisible to it, and the entry would never reach
 * `translation.json`. Keys there are extract-then-fill and never hand-written,
 * so the call has to be statically visible somewhere — here.
 *
 * Falls back to the template's English `defaultName` for an id with no case,
 * so adding a template can never render a raw key.
 */
export function templateName(t: TFunction, template: FrameTemplate): string {
  switch (template.id) {
    case "bar-id":
      return t("studio.frame.template.barId", { defaultValue: "Info Bar · Model" });
    case "bar-dark":
      return t("studio.frame.template.barDark", { defaultValue: "Info Bar · Dark" });
    case "bar-exif":
      return t("studio.frame.template.barExif", { defaultValue: "Info Bar · Settings" });
    case "bar-full":
      return t("studio.frame.template.barFull", { defaultValue: "Info Bar · Full" });
    case "margin-logo":
      return t("studio.frame.template.marginLogo", { defaultValue: "Margin · Centered Mark" });
    case "margin-gold":
      return t("studio.frame.template.marginGold", { defaultValue: "Margin · Gold" });
    case "border-stack":
      return t("studio.frame.template.borderStack", { defaultValue: "White Border · Centered" });
    case "gallery-split":
      return t("studio.frame.template.gallerySplit", { defaultValue: "Gallery · Justified" });
    case "margin-tb":
      return t("studio.frame.template.marginTopBottom", { defaultValue: "Margin · Top & Bottom" });
    case "text-margin":
      return t("studio.frame.template.textMargin", { defaultValue: "Margin · Model Only" });
    case "mag-cover":
      return t("studio.frame.template.magazineCover", { defaultValue: "Magazine Cover" });
    case "overlay-exif":
      return t("studio.frame.template.overlayExif", { defaultValue: "Overlay · Centered" });
    case "overlay-stack":
      return t("studio.frame.template.overlayStack", { defaultValue: "Overlay · Bottom Left" });
    case "overlay-corners":
      return t("studio.frame.template.overlayCorners", { defaultValue: "Overlay · Corners" });
    case "overlay-top":
      return t("studio.frame.template.overlayTop", { defaultValue: "Overlay · Top Mark" });
    case "overlay-min":
      return t("studio.frame.template.overlayMinimal", { defaultValue: "Overlay · Settings Only" });
    case "corner-logo":
      return t("studio.frame.template.cornerLogo", { defaultValue: "Overlay · Corner Mark" });
    case "dual-stack":
      return t("studio.frame.template.dualStack", { defaultValue: "Dual Mark · Centered" });
    case "dual-bar":
      return t("studio.frame.template.dualBar", { defaultValue: "Dual Mark · Info Bar" });
    case "vbar-right":
      return t("studio.frame.template.verticalRight", { defaultValue: "Vertical · Right Strip" });
    case "vbar-left":
      return t("studio.frame.template.verticalLeft", { defaultValue: "Vertical · Left Strip" });
    case "corner-vert":
      return t("studio.frame.template.cornerVertical", {
        defaultValue: "Wide Margin · Vertical Settings",
      });
    default:
      return template.defaultName;
  }
}
