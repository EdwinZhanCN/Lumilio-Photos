/**
 * The frame template format.
 *
 * A template is declarative data: a canvas treatment plus anchored elements.
 * It contains no rendering logic and produces no pixels. `expandTemplate` turns
 * one into a `CanvasSpec` and a list of ordinary layers, after which nothing
 * distinguishes template-produced content from something the user typed.
 *
 * One template renders ANY brand. A `logo` element resolves against whatever
 * brand the photo's EXIF matched, so these presets are brand-agnostic.
 *
 * ## Units
 *
 * Every fraction here is relative to the PHOTO's width, matching the basis the
 * layouts were originally designed and tuned against. The composed output is a
 * different size once padding is added, and `CanvasSpec`/`Layer` use different
 * bases again (short edge and output width respectively). `expandTemplate` is
 * the single place those conversions happen — no other module should rescale
 * template numbers.
 */

import type { CanvasBackground, CanvasPad, CanvasScrim } from "../../model/canvasSpec";
import type { FrameFontRole } from "../../model/fonts";
import type { ExifField } from "./frameExif";

export type TemplateFamily = "bar" | "margin" | "editorial" | "overlay" | "dual" | "vertical";

export type TemplateCanvas = {
  /** Padding per side, as fractions of the photo's width. Omitted sides are 0. */
  pad?: Partial<CanvasPad>;
  background: CanvasBackground;
  scrim?: CanvasScrim;
  /** Fractions of the relevant short edge; see `CanvasSpec`. */
  outerRadius?: number;
  innerRadius?: number;
  vignette?: number;
};

/**
 * Where an element sits.
 *
 * `region` picks which band: a padding band (`top`/`bottom`/`left`/`right`) or
 * the whole composed output (`full`, used by overlay templates that sit on the
 * photo). `h`/`v` place within the band — `v` accepts a number to address a
 * precise fraction of the band's height. `inset` is the margin from the
 * outer edges, and `dx`/`dy` nudge from there.
 */
export type TemplateAnchor = {
  region: "top" | "bottom" | "left" | "right" | "full";
  h: "left" | "center" | "right";
  v: "top" | "center" | "bottom" | number;
  inset?: number;
  dx?: number;
  dy?: number;
};

export type TemplateTextStyle = {
  font: FrameFontRole;
  weight?: number;
  italic?: boolean;
  /** Fraction of the photo's width. */
  size: number;
  color?: string;
  /** Letter spacing as a fraction of font size. */
  tracking?: number;
  opacity?: number;
  rotation?: number;
};

export type TemplateLogoStyle = {
  /**
   * Fraction of the photo's width, used as the mark's HEIGHT and then scaled by
   * the variant's `h` multiplier. Sizing by height is what lets one template
   * give a tall square symbol and a short wide wordmark the same visual weight.
   */
  size: number;
  opacity?: number;
  rotation?: number;
};

type ElementBase = {
  anchor: TemplateAnchor;
  /**
   * Elements sharing a group are vertically centered as a unit around their
   * shared anchor. When one line resolves to nothing — a fixed-lens camera has
   * no lens model — the rest stay centered instead of hanging to one side.
   */
  group?: string;
  /**
   * Anchor overrides applied when a dual-mark template resolved only one logo.
   * Not logo-specific: a layout that drops its second mark usually has to move
   * the accompanying text too.
   */
  soloAnchor?: Partial<TemplateAnchor>;
};

export type TemplateTextElement = ElementBase & {
  type: "text";
  /** May contain `{camera_model}`, `{lens_model}`, `{date}`, `{camera_make}`. */
  content: string;
  style: TemplateTextStyle;
};

export type TemplateExifElement = ElementBase & {
  type: "exif";
  fields: ExifField[];
  labeled?: boolean;
  separator?: string;
  style: TemplateTextStyle;
};

export type TemplateLogoElement = ElementBase & {
  type: "logo";
  variant?: string;
  kind?: "symbol" | "wordmark";
  /** Skip this slot when the brand lacks the variant, instead of falling back. */
  strict?: boolean;
  /** Template ink. A variant's own iconic color wins when this is neutral. */
  color?: string;
  style: TemplateLogoStyle;
};

export type TemplateElement = TemplateTextElement | TemplateExifElement | TemplateLogoElement;

export type FrameTemplate = {
  id: string;
  /** i18n key; `defaultName` is its English default for extraction. */
  nameKey: string;
  defaultName: string;
  family: TemplateFamily;
  canvas: TemplateCanvas;
  elements: TemplateElement[];
};
