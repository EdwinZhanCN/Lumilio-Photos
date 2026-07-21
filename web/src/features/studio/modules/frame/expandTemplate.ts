/**
 * Turns a frame template into a canvas spec and a list of ordinary layers.
 *
 * This is where a template stops being special. Everything it produces is a
 * normal `Layer` the user can move, retype, restyle, or delete, and nothing
 * downstream knows a template was involved.
 *
 * It is also the ONLY place template units are converted. Templates are
 * authored in fractions of the photo's width; `CanvasSpec.pad` is a fraction of
 * the short edge, and layer positions and sizes are fractions of the composed
 * output. Keeping all three conversions here is what lets the renderer stay
 * ignorant of anchors and alignment entirely.
 */

import {
  normalizeCanvasBackground,
  normalizeCanvasScrim,
  type CanvasSpec,
} from "../../model/canvasSpec";
import { FRAME_FONT_ROLES } from "../../model/fonts";
import { createLogoLayer, createTextLayer, type Layer, type TextLayer } from "../../model/layers";
import { measureTextLayer } from "../rendering/renderLayers";
import { formatExifLine, resolveTextTokens, type FrameExif } from "./frameExif";
import type {
  FrameTemplate,
  TemplateAnchor,
  TemplateElement,
  TemplateTextStyle,
} from "./frameTemplate";
import { matchBrand, pickVariant, resolveLogoColor, type LogoBrand } from "./logoRegistry";
import type { LogoRequest } from "./logoRaster";

export type ExpandContext = {
  photoWidth: number;
  photoHeight: number;
  exif: FrameExif;
  /**
   * The context used to measure text. It must have Studio's fonts loaded, and
   * should be the same kind of context the result will be drawn with.
   */
  measureCtx: OffscreenCanvasRenderingContext2D;
  /** User override for logo tint; color-locked marks ignore it. */
  logoColor?: string | null;
  /**
   * Average luminance (0..1) of a region of the already-composed background, in
   * output pixels. Overlay templates use it to pick legible ink. When absent
   * they assume a light mark on a dark scrim, which is what the scrim is for.
   */
  sampleLuminance?: (x: number, y: number, width: number, height: number) => number;
};

export type ExpandedTemplate = {
  canvas: CanvasSpec;
  layers: Layer[];
  /** Marks the renderer needs rasterized before it can draw. */
  logoRequests: LogoRequest[];
};

type Geometry = {
  /** Reference width all template fractions resolve against. */
  wref: number;
  photoHeight: number;
  padPx: { top: number; right: number; bottom: number; left: number };
  outWidth: number;
  outHeight: number;
};

type ResolvedAnchor = {
  x: number;
  y: number;
  align: "left" | "center" | "right";
};

function geometryFor(template: FrameTemplate, ctx: ExpandContext): Geometry {
  const wref = ctx.photoWidth;
  const pad = template.canvas.pad ?? {};
  const padPx = {
    top: (pad.top ?? 0) * wref,
    right: (pad.right ?? 0) * wref,
    bottom: (pad.bottom ?? 0) * wref,
    left: (pad.left ?? 0) * wref,
  };
  return {
    wref,
    photoHeight: ctx.photoHeight,
    padPx,
    outWidth: ctx.photoWidth + padPx.left + padPx.right,
    outHeight: ctx.photoHeight + padPx.top + padPx.bottom,
  };
}

/** Resolve an anchor to a point on the composed output, in pixels. */
function resolveAnchor(anchor: TemplateAnchor, geom: Geometry): ResolvedAnchor {
  const { wref, padPx, outWidth, outHeight } = geom;
  const inset = (anchor.inset ?? 0.05) * wref;
  const dx = (anchor.dx ?? 0) * wref;
  const dy = (anchor.dy ?? 0) * wref;

  // Side strips: a narrow left/right band. Content is centered across the strip
  // and positioned by `v` over the full height, since the band spans it.
  if (anchor.region === "left" || anchor.region === "right") {
    const isLeft = anchor.region === "left";
    const bandLeft = isLeft ? 0 : outWidth - padPx.right;
    const bandWidth = isLeft ? padPx.left : padPx.right;
    const hFraction = anchor.h === "left" ? 0.3 : anchor.h === "right" ? 0.7 : 0.5;
    const vFraction =
      typeof anchor.v === "number" ? anchor.v : anchor.v === "top" ? 0.12 : anchor.v === "bottom" ? 0.88 : 0.5;
    return {
      x: bandLeft + bandWidth * hFraction + dx,
      y: outHeight * vFraction + dy,
      align: "center",
    };
  }

  let bandTop: number;
  let bandHeight: number;
  if (anchor.region === "top") {
    bandTop = 0;
    bandHeight = padPx.top;
  } else if (anchor.region === "bottom") {
    bandTop = outHeight - padPx.bottom;
    bandHeight = padPx.bottom;
  } else {
    bandTop = 0;
    bandHeight = outHeight;
  }

  const vFraction =
    typeof anchor.v === "number" ? anchor.v : anchor.v === "top" ? 0.32 : anchor.v === "bottom" ? 0.68 : 0.5;
  const y = bandTop + bandHeight * vFraction + dy;

  if (anchor.h === "left") return { x: inset + dx, y, align: "left" };
  if (anchor.h === "right") return { x: outWidth - inset + dx, y, align: "right" };
  return { x: outWidth / 2 + dx, y, align: "center" };
}

/** The text an element resolves to, or "" when the photo lacks the data. */
function elementText(element: TemplateElement, exif: FrameExif): string {
  if (element.type === "text") return resolveTextTokens(element.content, exif);
  if (element.type === "exif") {
    return formatExifLine(exif, element.fields, {
      labeled: element.labeled,
      separator: element.separator,
    });
  }
  return "";
}

/**
 * Vertical offsets that center each group's surviving lines as a unit.
 *
 * A template stacking model over lens expects two lines. A fixed-lens body
 * (Ricoh GR, Fuji X100) has no lens model, so without this the remaining line
 * would sit where the top of a two-line stack was, visibly high.
 */
function groupOffsets(template: FrameTemplate, exif: FrameExif): Map<number, number> {
  const stacks = new Map<string, number[]>();
  template.elements.forEach((element, index) => {
    if (!element.group || element.type === "logo") return;
    if (!elementText(element, exif)) return;
    const members = stacks.get(element.group);
    if (members) members.push(index);
    else stacks.set(element.group, [index]);
  });

  const offsets = new Map<number, number>();
  for (const members of stacks.values()) {
    const lineHeights = members.map((index) => {
      const element = template.elements[index];
      const style = (element as { style: TemplateTextStyle }).style;
      return style.size * 1.6;
    });
    const total = lineHeights.reduce((sum, height) => sum + height, 0);
    let cursor = -total / 2;
    members.forEach((index, i) => {
      offsets.set(index, cursor + lineHeights[i] / 2);
      cursor += lineHeights[i];
    });
  }
  return offsets;
}

function buildTextLayer(
  text: string,
  style: TemplateTextStyle,
  anchor: ResolvedAnchor,
  geom: Geometry,
  ctx: ExpandContext,
  isOverlay: boolean,
): TextLayer {
  const fontPx = style.size * geom.wref;

  let color = style.color ?? "#141414";
  let shadow: TextLayer["shadow"] = null;

  if (isOverlay) {
    // On-photo text picks black or white by what is behind it, so it survives
    // both a bright sky and a dark shadow, and takes an opposite-colored halo.
    let dark = true;
    if (ctx.sampleLuminance) {
      const luminance = ctx.sampleLuminance(
        anchor.x - geom.outWidth * 0.17,
        anchor.y - fontPx * 0.8,
        geom.outWidth * 0.34,
        fontPx * 1.6,
      );
      dark = luminance < 0.55;
    }
    color = dark ? "#ffffff" : "#141414";
    shadow = {
      color: dark ? "#000000" : "#ffffff",
      opacity: 0.38,
      blur: (fontPx * 0.4) / geom.outWidth,
      offsetX: 0,
      offsetY: 0,
    };
  }

  const layer = createTextLayer({
    text,
    font: {
      family: FRAME_FONT_ROLES[style.font],
      weight: style.weight ?? 400,
      italic: style.italic ?? false,
      size: fontPx / geom.outWidth,
      tracking: style.tracking ?? 0,
      lineHeight: 1.2,
    },
    align: "center",
    fill: { kind: "solid", color, opacity: 1 },
    opacity: style.opacity ?? 1,
    rotation: style.rotation ?? 0,
    shadow,
    fromTemplate: true,
    x: anchor.x / geom.outWidth,
    y: anchor.y / geom.outHeight,
  });

  // The anchor names an EDGE for left/right alignment, but a layer is placed by
  // its center. Measure and shift, so the renderer never has to know about
  // alignment. Rotated text measures along its own axis, so the shift would be
  // wrong — those are always center-anchored in the templates anyway.
  if (anchor.align !== "center" && !layer.rotation) {
    const width = measureTextLayer(ctx.measureCtx, layer, geom.outWidth).width;
    const direction = anchor.align === "left" ? 1 : -1;
    layer.x = (anchor.x + (direction * width) / 2) / geom.outWidth;
  }

  return layer;
}

export function expandTemplate(template: FrameTemplate, ctx: ExpandContext): ExpandedTemplate {
  const geom = geometryFor(template, ctx);
  const shortEdge = Math.min(ctx.photoWidth, ctx.photoHeight);
  const isOverlay = template.family === "overlay";

  const canvas: CanvasSpec = {
    pad: {
      top: geom.padPx.top / shortEdge,
      right: geom.padPx.right / shortEdge,
      bottom: geom.padPx.bottom / shortEdge,
      left: geom.padPx.left / shortEdge,
    },
    background: normalizeCanvasBackground(template.canvas.background),
    outerRadius: template.canvas.outerRadius ?? 0,
    innerRadius: template.canvas.innerRadius ?? 0,
    scrim: template.canvas.scrim ? normalizeCanvasScrim(template.canvas.scrim) : null,
    vignette: template.canvas.vignette ?? 0,
  };

  const brand: LogoBrand | null = matchBrand(ctx.exif.make, ctx.exif.model);

  // Dual-mark templates degrade to a single centered mark when the brand only
  // has one usable variant, via each element's `soloAnchor`.
  let resolvedMarks = 0;
  if (template.family === "dual" && brand) {
    for (const element of template.elements) {
      if (element.type !== "logo") continue;
      if (pickVariant(brand, { variantId: element.variant, kind: element.kind, strict: element.strict })) {
        resolvedMarks += 1;
      }
    }
  }
  const solo = template.family === "dual" && resolvedMarks === 1;

  const offsets = groupOffsets(template, ctx.exif);
  const layers: Layer[] = [];
  const logoRequests: LogoRequest[] = [];

  template.elements.forEach((element, index) => {
    let anchorDef = element.anchor;
    if (solo && element.soloAnchor) {
      anchorDef = { ...anchorDef, ...element.soloAnchor };
    }
    const groupDy = offsets.get(index);
    if (groupDy !== undefined) {
      anchorDef = { ...anchorDef, dy: (anchorDef.dy ?? 0) + groupDy };
    }
    const anchor = resolveAnchor(anchorDef, geom);

    if (element.type === "logo") {
      if (!brand) return;
      const variant = pickVariant(brand, {
        variantId: element.variant,
        kind: element.kind,
        strict: element.strict,
      });
      if (!variant) return;

      const color = resolveLogoColor(variant, element.color ?? null, ctx.logoColor ?? null);

      // Marks are sized by HEIGHT times the variant's normalizing multiplier,
      // then converted to the width fraction the renderer wants. That is what
      // makes one template give a tall symbol and a wide wordmark equal weight.
      const heightPx = element.style.size * variant.h * geom.wref;
      const widthPx = heightPx * variant.aspect;

      // A wide wordmark cannot fit across a narrow side strip; rotate it to
      // read vertically. Square-ish marks stay upright.
      const inStrip = anchorDef.region === "left" || anchorDef.region === "right";
      const rotation = inStrip && variant.aspect > 1.6 ? 90 : (element.style.rotation ?? 0);

      let centerX = anchor.x;
      if (anchor.align === "left") centerX = anchor.x + widthPx / 2;
      else if (anchor.align === "right") centerX = anchor.x - widthPx / 2;

      logoRequests.push({ brand: brand.id, variant: variant.id, color });
      layers.push(
        createLogoLayer(brand.id, variant.id, {
          color,
          size: widthPx / geom.outWidth,
          x: centerX / geom.outWidth,
          y: anchor.y / geom.outHeight,
          rotation,
          opacity: element.style.opacity ?? 1,
          // A light mark on a bright photo needs separation; the cached bitmap
          // cannot be re-tinted per region, so give it a soft shadow instead.
          shadow: isOverlay
            ? { color: "#000000", opacity: 0.45, blur: 0.005, offsetX: 0, offsetY: 0 }
            : null,
          fromTemplate: true,
        }),
      );
      return;
    }

    const text = elementText(element, ctx.exif);
    if (!text) return; // no value for this photo — drop the line entirely
    layers.push(buildTextLayer(text, element.style, anchor, geom, ctx, isOverlay));
  });

  return { canvas, layers, logoRequests };
}
