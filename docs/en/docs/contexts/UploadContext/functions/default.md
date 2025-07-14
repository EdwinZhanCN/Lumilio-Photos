[lumilio-web](../../../modules.md) / [contexts/UploadContext](../index.md) / default

# Function: default()

> **default**(`props`): `Element`

Defined in: [contexts/UploadContext.tsx:321](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/contexts/UploadContext.tsx#L321)

**Upload Provider Component**

Main provider component that manages upload state and provides context to child components.
Handles WASM initialization, state management, and coordinates upload operations.

## Parameters

### props

`UploadProviderProps`

Provider props containing children

## Returns

`Element`

JSX element wrapping children with upload context

## Example

```tsx
// At the root of your application
function App() {
  return (
    <UploadProvider>
      <Header />
      <MainContent />
      <Footer />
    </UploadProvider>
  );
}

// In any child component
function FileUploadZone() {
  const { state, BatchUpload } = useUploadContext();
  // Component implementation...
}
```

## Since

1.0.0
