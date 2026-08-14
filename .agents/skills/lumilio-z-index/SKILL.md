---
name: lumilio-z-index
description: Use when adding or changing overlapping UI in Lumilio Photos —
  decorative overlays, component-internal stacking, or a new cross-component
  floating layer. Apply the three-rule z-index strategy and the token scale;
  never invent a new numeric z-index for a global layer.
---

# Apply The Z-Index Strategy

Three rules, in priority order. The token table lives here so a session can
apply it without opening FRONTEND.md; the design intent (calm operational UI,
no competing stacks) is owned by [DESIGN.md](../../../site/docs/internal/DESIGN.md).

## 1. Decorative overlays → DOM order

Gradient tints, badges, hover masks, and other purely visual layers must not
carry a z-index. Place them after the content they cover in DOM order.

## 2. Component-internal overlap → `isolation: isolate`

When a component has multiple overlapping layers (dropdown inside a card,
sticky header inside a panel), add `isolate` to the component root and keep
internal values small (`z-10` / `z-20` / `z-30`). Internal z-index must never
leak into the global stack.

## 3. Cross-component floating layers → z-index tokens

Use the theme tokens defined in `web/src/styles/App.css` `@theme inline`:

| Token | Value | Use |
| --- | --- | --- |
| `z-sticky` | 100 | Sticky headers, save bars |
| `z-dropdown` | 200 | Dropdown menus, popovers, autocomplete |
| `z-overlay` | 300 | FABs, application drawers, floating docks, drag overlays |
| `z-modal` | 400 | Modals and modal bottom-sheets |
| `z-lightbox` | 500 | Fullscreen viewers (AssetViewer, PublicShareLightbox) |
| `z-tooltip` | 600 | Portaled tooltips/popovers that escape a lightbox |
| `z-toast` | 700 | App-wide notifications above other document-layer floating UI |

Apply a global token once, on the floating layer's stacking-context root. Its
children should use DOM order or small local values. If an in-tree floating
layer is trapped by a component's `isolate`, portal the layer root to
`document.body`.

React-controlled daisyUI `.modal-open` roots must carry `z-modal`, which
overrides daisyUI's library default. Native dialogs opened with `showModal()`
live in the browser top layer and are the exception: do not add a token merely
to compete with document stacking contexts.

Inline styles use `var(--z-index-<token-name>)`. Do not introduce new numeric
z-index values for cross-component layers; extend the token scale in `App.css`
if a new tier is genuinely needed, and update this table in the same change.

## Verify

Search the diff for raw `z-index:` / `z-[` / `z-50` and above that are not
tokens or the small internal `z-10`/`z-20`/`z-30` band. A new global number
is a miss.
