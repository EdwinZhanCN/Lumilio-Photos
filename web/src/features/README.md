# Features and State Management

This directory contains the core business logic and UI modules of the application. Each feature is self-contained and follows a consistent structure for state management and UI design.

## State Management Structure

We follow a **feature-based state management** approach. While global state exists, we encourage using Redux on a per-feature basis to keep the logic modular and maintainable.

### Feature Store Pattern

Each feature should manage its own state. Feel free to use Redux, but ensure it is scoped to the feature: **one store per feature**.

Example structure for a `settings` feature:

```txt
src/
└── features/
    └── settings/
        ├── index.ts              // Entry point: export { SettingsProvider, useSettings } from '...'
        ├── SettingsProvider.tsx  // Provider component (wraps Redux store or Context)
        ├── hooks/
        │   └── useSettings.ts    // Custom Hook for accessing feature state
        ├── reducers/
        │   ├── lumen.reducer.ts  // Sub-reducer
        │   └── ui.reducer.ts     // Sub-reducer
        ├── settings.reducer.ts   // Root Reducer for this feature
        └── settings.assets.auth.collections.settings.upload.types.ts              // Feature-specific type definitions
```

## Design Protocol

### Dependencies
- TailwindCSS
- DaisyUI

### Header
- The header should be consistent across all pages.
- It should include the icon on the left and navigation title on the right.

Using Heroicons or Lucide-react components for the icon is recommended. Standard className should be `w-6 h-6 text-primary` or `size-6 text-primary`.

```jsx
<PageHeader
  title="Studio"
  icon={<PaintBrushIcon className="w-6 h-6 text-primary" />}
/>
```

### Buttons
- We prefer the **soft button style** from DaisyUI.
- For frequently used buttons, use: `className="btn btn-sm btn-soft btn-info"`.
