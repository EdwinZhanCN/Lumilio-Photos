# Lumilio Docs Site

This directory contains the public Lumilio Photos documentation site. It is a VitePress site managed with Vite+ package commands.

## Local Commands

```bash
vp install
vp run dev
vp run build
vp run preview
```

Refresh the generated API reference from the root workspace:

```bash
make dto
```

The VitePress source root is `docs/`. Production output is generated at:

```text
docs/.vitepress/dist
```
