/**
 * The built-in frame templates.
 *
 * Layouts and their tuned proportions are ported from AfterFrame, verified
 * against Hasselblad, Leica, Canon, and Fujifilm bodies. Names are i18n keys
 * rather than literals (the originals were hardcoded Chinese).
 *
 * All numbers are fractions of the PHOTO's width — see `frameTemplate.ts`.
 * `expandTemplate` converts them; do not rescale them anywhere else.
 */

import type { FrameTemplate } from "./frameTemplate";

const INK = "#1a1a1a";
const SUB = "#9a9a9a";
const WHITE = "#f4f4f4";

const white = { kind: "solid", color: "#ffffff" } as const;

/** Overlay templates sit on the photo, so they need a scrim under their text. */
const scrimBottom = (height: number, to: string) =>
  ({ edge: "bottom", from: "rgba(0,0,0,0)", to, height }) as const;

export const FRAME_TEMPLATES: readonly FrameTemplate[] = [
  // ───────────────────────────── Info bar ─────────────────────────────
  {
    id: "bar-id",
    nameKey: "studio.frame.template.barId",
    defaultName: "Info Bar · Model",
    family: "bar",
    canvas: { pad: { bottom: 0.122 }, background: white },
    elements: [
      {
        type: "text",
        content: "{camera_model}",
        group: "info",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.024, color: INK },
      },
      {
        type: "text",
        content: "{lens_model}",
        group: "info",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.0165, color: SUB },
      },
      {
        type: "logo",
        variant: "symbol",
        anchor: { region: "bottom", h: "right", v: "center", inset: 0.05 },
        style: { size: 0.062 },
      },
    ],
  },
  {
    id: "bar-dark",
    nameKey: "studio.frame.template.barDark",
    defaultName: "Info Bar · Dark",
    family: "bar",
    canvas: { pad: { bottom: 0.122 }, background: { kind: "solid", color: "#0c0c0c" } },
    elements: [
      {
        type: "text",
        content: "{camera_model}",
        group: "info",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.024, color: WHITE },
      },
      {
        type: "text",
        content: "{lens_model}",
        group: "info",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.0165, color: "#8a8a8a" },
      },
      {
        type: "logo",
        variant: "symbol",
        color: WHITE,
        anchor: { region: "bottom", h: "right", v: "center", inset: 0.05 },
        style: { size: 0.062 },
      },
    ],
  },
  {
    id: "bar-exif",
    nameKey: "studio.frame.template.barExif",
    defaultName: "Info Bar · Settings",
    family: "bar",
    canvas: { pad: { bottom: 0.118 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.05 },
        style: { size: 0.055 },
      },
      {
        type: "exif",
        labeled: true,
        fields: ["aperture", "shutter", "iso"],
        anchor: { region: "bottom", h: "right", v: "center", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.0165, color: "#4a4a4a" },
      },
    ],
  },
  {
    id: "bar-full",
    nameKey: "studio.frame.template.barFull",
    defaultName: "Info Bar · Full",
    family: "bar",
    canvas: { pad: { bottom: 0.138 }, background: white },
    elements: [
      {
        type: "text",
        content: "{camera_model}",
        group: "info",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.022, color: INK },
      },
      {
        type: "text",
        content: "{lens_model}",
        group: "info",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.0155, color: SUB },
      },
      {
        type: "logo",
        variant: "symbol",
        anchor: { region: "bottom", h: "right", v: "top", inset: 0.05 },
        style: { size: 0.05 },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "bottom", h: "right", v: "bottom", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.0145, color: "#8a8a8a" },
      },
    ],
  },

  // ───────────────────────────── Margin ─────────────────────────────
  {
    id: "margin-logo",
    nameKey: "studio.frame.template.marginLogo",
    defaultName: "Margin · Centered Mark",
    family: "margin",
    canvas: { pad: { top: 0.05, left: 0.05, right: 0.05, bottom: 0.15 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "wordmark",
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: -0.012 },
        style: { size: 0.085 },
      },
      {
        type: "exif",
        labeled: true,
        fields: ["aperture", "shutter", "iso"],
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: 0.04 },
        style: { font: "grotesk", weight: 400, size: 0.0155, color: "#6e6e6e" },
      },
    ],
  },
  {
    id: "margin-gold",
    nameKey: "studio.frame.template.marginGold",
    defaultName: "Margin · Gold",
    family: "margin",
    canvas: {
      pad: { top: 0.05, left: 0.05, right: 0.05, bottom: 0.14 },
      background: { kind: "solid", color: "#fbfaf7" },
    },
    elements: [
      {
        type: "logo",
        variant: "wordmark",
        color: "#b08d4c",
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05 },
        style: { size: 0.085 },
      },
    ],
  },
  {
    id: "border-stack",
    nameKey: "studio.frame.template.borderStack",
    defaultName: "White Border · Centered",
    family: "margin",
    canvas: { pad: { top: 0.045, left: 0.045, right: 0.045, bottom: 0.2 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: -0.052 },
        style: { size: 0.06 },
      },
      {
        type: "text",
        content: "{camera_model}",
        group: "info",
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: 0.032 },
        style: { font: "grotesk", weight: 400, size: 0.022, color: INK },
      },
      {
        type: "text",
        content: "{lens_model}",
        group: "info",
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: 0.032 },
        style: { font: "grotesk", weight: 400, size: 0.016, color: "#a0a0a0" },
      },
    ],
  },
  {
    id: "gallery-split",
    nameKey: "studio.frame.template.gallerySplit",
    defaultName: "Gallery · Justified",
    family: "margin",
    canvas: { pad: { top: 0.05, left: 0.05, right: 0.05, bottom: 0.12 }, background: white },
    elements: [
      {
        type: "text",
        content: "{camera_model}",
        group: "info",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.055 },
        style: { font: "grotesk", weight: 400, size: 0.02, color: INK },
      },
      {
        type: "text",
        content: "{lens_model}",
        group: "info",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.055 },
        style: { font: "grotesk", weight: 400, size: 0.0145, color: SUB },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "bottom", h: "right", v: "center", inset: 0.055 },
        style: { font: "grotesk", weight: 400, size: 0.0145, color: "#8a8a8a" },
      },
    ],
  },
  {
    id: "margin-tb",
    nameKey: "studio.frame.template.marginTopBottom",
    defaultName: "Margin · Top & Bottom",
    family: "margin",
    canvas: { pad: { top: 0.1, left: 0.05, right: 0.05, bottom: 0.1 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "wordmark",
        anchor: { region: "top", h: "center", v: "center", inset: 0.05 },
        style: { size: 0.07 },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.015, color: "#6e6e6e", tracking: 0.06 },
      },
    ],
  },
  {
    id: "text-margin",
    nameKey: "studio.frame.template.textMargin",
    defaultName: "Margin · Model Only",
    family: "margin",
    canvas: { pad: { top: 0.05, left: 0.05, right: 0.05, bottom: 0.12 }, background: white },
    elements: [
      {
        type: "text",
        content: "{camera_model}",
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: -0.01 },
        style: { font: "grotesk", weight: 400, size: 0.022, color: INK },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: 0.032 },
        style: { font: "grotesk", weight: 400, size: 0.0135, color: SUB, tracking: 0.05 },
      },
    ],
  },
  {
    id: "mag-cover",
    nameKey: "studio.frame.template.magazineCover",
    defaultName: "Magazine Cover",
    family: "editorial",
    canvas: { pad: { top: 0.05, left: 0.05, right: 0.05, bottom: 0.145 }, background: white },
    elements: [
      {
        type: "text",
        content: "{camera_model}",
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.055, dy: -0.012 },
        style: { font: "grotesk", weight: 300, size: 0.034, color: INK },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.055, dy: 0.034 },
        style: { font: "grotesk", weight: 400, size: 0.0125, color: SUB, tracking: 0.04 },
      },
      {
        type: "logo",
        variant: "symbol",
        anchor: { region: "bottom", h: "right", v: "center", inset: 0.055 },
        style: { size: 0.072 },
      },
    ],
  },

  // ───────────────────────────── Overlay ─────────────────────────────
  {
    id: "overlay-exif",
    nameKey: "studio.frame.template.overlayExif",
    defaultName: "Overlay · Centered",
    family: "overlay",
    canvas: { background: white, scrim: scrimBottom(0.32, "rgba(0,0,0,0.5)") },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        color: "#ffffff",
        anchor: { region: "full", h: "center", v: 0.86 },
        style: { size: 0.05, opacity: 0.95 },
      },
      {
        type: "exif",
        labeled: true,
        fields: ["aperture", "shutter", "iso"],
        anchor: { region: "full", h: "center", v: 0.94 },
        style: {
          font: "grotesk",
          weight: 400,
          size: 0.016,
          color: "#ffffff",
          tracking: 0.02,
          opacity: 0.92,
        },
      },
    ],
  },
  {
    id: "overlay-stack",
    nameKey: "studio.frame.template.overlayStack",
    defaultName: "Overlay · Bottom Left",
    family: "overlay",
    canvas: { background: white, scrim: scrimBottom(0.42, "rgba(0,0,0,0.55)") },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        color: "#ffffff",
        anchor: { region: "full", h: "left", v: 0.78, inset: 0.055 },
        style: { size: 0.06, opacity: 0.95 },
      },
      {
        type: "text",
        content: "{camera_model}",
        anchor: { region: "full", h: "left", v: 0.89, inset: 0.055 },
        style: { font: "grotesk", weight: 400, size: 0.022, color: "#ffffff" },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "full", h: "left", v: 0.95, inset: 0.055 },
        style: { font: "grotesk", weight: 400, size: 0.014, color: "rgba(255,255,255,0.85)" },
      },
    ],
  },
  {
    id: "overlay-corners",
    nameKey: "studio.frame.template.overlayCorners",
    defaultName: "Overlay · Corners",
    family: "overlay",
    canvas: { background: white, scrim: scrimBottom(0.4, "rgba(0,0,0,0.45)") },
    elements: [
      {
        type: "text",
        content: "{date}",
        anchor: { region: "full", h: "left", v: 0.07, inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.014, color: "rgba(255,255,255,0.92)" },
      },
      {
        type: "logo",
        variant: "symbol",
        color: "#ffffff",
        anchor: { region: "full", h: "right", v: 0.075, inset: 0.05 },
        style: { size: 0.045, opacity: 0.95 },
      },
      {
        type: "text",
        content: "{camera_model}",
        anchor: { region: "full", h: "left", v: 0.93, inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.02, color: "#ffffff" },
      },
      {
        type: "exif",
        fields: ["aperture", "shutter", "iso"],
        anchor: { region: "full", h: "right", v: 0.93, inset: 0.05 },
        style: { font: "grotesk", weight: 400, size: 0.014, color: "rgba(255,255,255,0.85)" },
      },
    ],
  },
  {
    id: "overlay-top",
    nameKey: "studio.frame.template.overlayTop",
    defaultName: "Overlay · Top Mark",
    family: "overlay",
    canvas: {
      background: white,
      scrim: { edge: "top", from: "rgba(0,0,0,0)", to: "rgba(0,0,0,0.45)", height: 0.3 },
    },
    elements: [
      {
        type: "logo",
        variant: "wordmark",
        color: "#ffffff",
        anchor: { region: "full", h: "center", v: 0.08 },
        style: { size: 0.05, opacity: 0.95 },
      },
    ],
  },
  {
    id: "overlay-min",
    nameKey: "studio.frame.template.overlayMinimal",
    defaultName: "Overlay · Settings Only",
    family: "overlay",
    canvas: { background: white, scrim: scrimBottom(0.28, "rgba(0,0,0,0.5)") },
    elements: [
      {
        type: "exif",
        labeled: true,
        fields: ["aperture", "shutter", "iso"],
        anchor: { region: "full", h: "center", v: 0.93 },
        style: {
          font: "grotesk",
          weight: 400,
          size: 0.016,
          color: "#ffffff",
          tracking: 0.02,
          opacity: 0.92,
        },
      },
    ],
  },
  {
    id: "corner-logo",
    nameKey: "studio.frame.template.cornerLogo",
    defaultName: "Overlay · Corner Mark",
    family: "overlay",
    canvas: { background: white, scrim: scrimBottom(0.24, "rgba(0,0,0,0.4)") },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        color: "#ffffff",
        anchor: { region: "full", h: "right", v: 0.9, inset: 0.05 },
        style: { size: 0.07, opacity: 0.96 },
      },
    ],
  },

  // ───────────────────────────── Dual mark ─────────────────────────────
  {
    id: "dual-stack",
    nameKey: "studio.frame.template.dualStack",
    defaultName: "Dual Mark · Centered",
    family: "dual",
    canvas: { pad: { top: 0.05, left: 0.05, right: 0.05, bottom: 0.16 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        strict: true,
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: -0.03 },
        soloAnchor: { dy: 0 },
        style: { size: 0.062 },
      },
      {
        type: "logo",
        variant: "wordmark",
        strict: true,
        anchor: { region: "bottom", h: "center", v: "center", inset: 0.05, dy: 0.04 },
        soloAnchor: { dy: 0 },
        style: { size: 0.06 },
      },
    ],
  },
  {
    id: "dual-bar",
    nameKey: "studio.frame.template.dualBar",
    defaultName: "Dual Mark · Info Bar",
    family: "dual",
    canvas: { pad: { bottom: 0.122 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "wordmark",
        strict: true,
        anchor: { region: "bottom", h: "left", v: "center", inset: 0.05 },
        style: { size: 0.05 },
      },
      {
        type: "logo",
        variant: "symbol",
        strict: true,
        anchor: { region: "bottom", h: "right", v: "top", inset: 0.05 },
        soloAnchor: { h: "left", v: "center" },
        style: { size: 0.05 },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "bottom", h: "right", v: "bottom", inset: 0.05 },
        soloAnchor: { v: "center" },
        style: { font: "grotesk", weight: 400, size: 0.0145, color: "#8a8a8a" },
      },
    ],
  },

  // ───────────────────────────── Vertical strip ─────────────────────────────
  {
    id: "vbar-right",
    nameKey: "studio.frame.template.verticalRight",
    defaultName: "Vertical · Right Strip",
    family: "vertical",
    canvas: { pad: { right: 0.12 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        anchor: { region: "right", h: "center", v: 0.1 },
        style: { size: 0.05 },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "right", h: "center", v: 0.58 },
        style: {
          font: "grotesk",
          weight: 400,
          size: 0.013,
          color: "#8a8a8a",
          tracking: 0.08,
          rotation: 90,
        },
      },
    ],
  },
  {
    id: "vbar-left",
    nameKey: "studio.frame.template.verticalLeft",
    defaultName: "Vertical · Left Strip",
    family: "vertical",
    canvas: { pad: { left: 0.11 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        anchor: { region: "left", h: "center", v: 0.1 },
        style: { size: 0.05 },
      },
      {
        type: "text",
        content: "{camera_model}",
        anchor: { region: "left", h: "center", v: 0.6 },
        style: { font: "grotesk", weight: 400, size: 0.016, color: INK, rotation: 90 },
      },
    ],
  },
  {
    id: "corner-vert",
    nameKey: "studio.frame.template.cornerVertical",
    defaultName: "Wide Margin · Vertical Settings",
    family: "vertical",
    canvas: { pad: { top: 0.07, left: 0.06, right: 0.1, bottom: 0.06 }, background: white },
    elements: [
      {
        type: "logo",
        variant: "symbol",
        anchor: { region: "top", h: "left", v: "center", inset: 0.05 },
        style: { size: 0.05 },
      },
      {
        type: "exif",
        fields: ["focal", "aperture", "shutter", "iso"],
        anchor: { region: "right", h: "center", v: 0.5 },
        style: {
          font: "grotesk",
          weight: 400,
          size: 0.0125,
          color: "#8a8a8a",
          tracking: 0.06,
          rotation: 90,
        },
      },
    ],
  },
];

export const TEMPLATES_BY_ID = new Map<string, FrameTemplate>(
  FRAME_TEMPLATES.map((template) => [template.id, template]),
);

export function findTemplate(id: string): FrameTemplate | null {
  return TEMPLATES_BY_ID.get(id) ?? null;
}
