# DMG window artwork (optional)

`desktop/scripts/build-macos.sh ... --dmg` uses
[`create-dmg`](https://github.com/create-dmg/create-dmg) (`brew install create-dmg`)
to build the classic "drag the app onto Applications" window:

```
┌─────────────────────────────────────────────┐
│                Lumilio Photos                 │
│                                               │
│     [App icon]   ──────────▶   [Applications] │
│      (165,200)                    (495,200)    │
└─────────────────────────────────────────────┘
        window 660×400, icon size 120
```

The Applications symlink and icon/window positions are always set. A background
image is **optional polish** (the arrow + branding behind the icons):

- Put a PNG at **`background.png`** in this directory. The build picks it up
  automatically; without it the DMG still shows the two positioned icons.
- Size it to the window: **660×400** px. For crisp Retina rendering, supply a
  2×-resolution image and let `create-dmg` handle scaling, or keep a simple flat
  background where scaling is unnoticeable.
- Design so the app icon sits around x≈165 and the Applications folder around
  x≈495 (the arrow points between them). Keep these in sync with the `--icon` /
  `--app-drop-link` coordinates in `build-macos.sh` if you change the layout.

Styling the window relies on Finder/AppleScript, so it needs a GUI session
(a local Mac, or a CI macOS runner). If that step fails, the build falls back to
a plain DMG that still contains the Applications symlink (functional drag-drop,
no custom window).
