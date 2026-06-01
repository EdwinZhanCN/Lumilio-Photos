# Lumilio Photos - Web Frontend

The modern, high-performance web interface for Lumilio Photos, built with React, TypeScript, and WebAssembly.

## Tech Stack

- **Framework:** [React 19](https://react.dev/)
- **Build Toolchain:** [Vite+](https://viteplus.dev/) (Vite 8 core)
- **Language:** [TypeScript](https://www.typescriptlang.org/)
- **Styling:** [Tailwind CSS](https://tailwindcss.com/) & [DaisyUI](https://daisyui.com/)
- **State Management:** [Zustand](https://zustand-demo.pmnd.rs/) & [TanStack Query](https://tanstack.com/query/latest)
- **Performance:** [WebAssembly (WASM)](https://webassembly.org/) & [Web Workers](https://developer.mozilla.org/en-US/docs/Web/API/Web_Workers_API)
- **Routing:** [React Router 7](https://reactrouter.com/)
- **Testing:** Vite+ test runner (Vitest-compatible APIs)

## Getting Started

### Prerequisites

- [Node.js](https://nodejs.org/) (version specified in `.nvmrc`)
- [Vite+ `vp`](https://viteplus.dev/) (delegates installs through pnpm)

### Installation

```bash
vp install
```

### Development

```bash
vp dev
```

### Build

```bash
vp build
```

### Other Scripts

- `vp lint`: Run Oxlint through Vite+.
- `vp check --no-fmt --no-lint`: Run TypeScript type checking through Vite+.
- `vp test`: Run unit tests through Vite+.

## Project Structure

The project follows a feature-based and modular architecture:

- **`src/features/`**: Domain-specific logic and components (e.g., `auth`, `home`).
- **`src/wasm/`**: WebAssembly modules for heavy computations (Exif extraction, image processing, hashing).
- **`src/workers/`**: Web Workers to run WASM and other intensive tasks off the main thread.
- **`src/hooks/`**: `util-hooks/` for reusable UI and logic hooks.
- **`src/lib/`**: Core utilities, OpenAPI-based HTTP client and React Query setup, i18n configuration, and shared helpers.
- **`src/contexts/`**: Global React Contexts for state like `WorkerProvider` and `GlobalContext`.
- **`src/components/`**: Reusable UI components.
- **`src/styles/`**: Global CSS and Tailwind configurations.

## Key Features

- **Client-side Processing:** Uses WASM (Blake3, Exiv2) to process images and metadata directly in the browser.
- **Multithreaded:** Offloads heavy tasks to Web Workers to ensure a smooth 60fps UI.
- **Justified Layout:** Efficient photo grid rendering using `@immich/justified-layout-wasm`.
- **Internationalization:** Full i18n support using `react-i18next`.
- **Modern UI:** Responsive design with Tailwind CSS

## Testing

We use the Vite+ test command for unit and integration testing.

```bash
vp test                # Run tests
vp test --ui           # Run tests with UI
vp test run --coverage # Generate coverage report
```


