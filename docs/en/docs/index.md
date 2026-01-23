# Lumilio Photos - Web Frontend

The modern, high-performance web interface for Lumilio Photos, built with React, TypeScript, and WebAssembly.

## Tech Stack

- **Framework:** [React 19](https://react.dev/)
- **Build Tool:** [Vite](https://vitejs.dev/)
- **Language:** [TypeScript](https://www.typescriptlang.org/)
- **Styling:** [Tailwind CSS](https://tailwindcss.com/) & [DaisyUI](https://daisyui.com/)
- **State Management:** [Redux Toolkit](https://redux-toolkit.js.org/) & [TanStack Query](https://tanstack.com/query/latest)
- **Performance:** [WebAssembly (WASM)](https://webassembly.org/) & [Web Workers](https://developer.mozilla.org/en-US/docs/Web/API/Web_Workers_API)
- **Routing:** [React Router 7](https://reactrouter.com/)
- **Testing:** [Vitest](https://vitest.dev/)

## Getting Started

### Prerequisites

- [Node.js](https://nodejs.org/) (version specified in `.nvmrc`)
- [pnpm](https://pnpm.io/) (recommended)

### Installation

```bash
pnpm install
```

### Development

```bash
pnpm dev
```

### Build

```bash
pnpm build
```

### Other Scripts

- `pnpm lint`: Run oxlint for fast linting.
- `pnpm type-check`: Run TypeScript type checking.
- `pnpm test`: Run unit tests with Vitest.
- `pnpm docs`: Generate documentation using TypeDoc.

## Project Structure

The project follows a feature-based and modular architecture:

- **`src/features/`**: Domain-specific logic and components (e.g., `auth`, `home`).
- **`src/wasm/`**: WebAssembly modules for heavy computations (Exif extraction, image processing, hashing).
- **`src/workers/`**: Web Workers to run WASM and other intensive tasks off the main thread.
- **`src/hooks/`**: 
    - `api-hooks/`: Data fetching and mutation hooks.
    - `util-hooks/`: Reusable UI and logic hooks.
- **`src/lib/`**: Core utilities, HTTP client (Axios), i18n configuration, and shared helper functions.
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

We use Vitest for unit and integration testing.

```bash
pnpm test          # Run tests
pnpm test:ui       # Run tests with UI
pnpm test:coverage # Generate coverage report
```

## Documentation

API and internal documentation can be generated via TypeDoc:

```bash
pnpm docs
```
