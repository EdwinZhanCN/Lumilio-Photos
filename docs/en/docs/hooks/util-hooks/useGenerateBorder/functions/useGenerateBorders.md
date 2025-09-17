[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateBorder](../index.md) / useGenerateBorders

# Function: useGenerateBorders()

> **useGenerateBorders**(): [`UseGenerateBordersReturn`](../interfaces/UseGenerateBordersReturn.md)

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:67](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/bdb61d82271cd56f7d31e6f3e50cded728e37cea/web/src/hooks/util-hooks/useGenerateBorder.tsx#L67)

Custom hook to generate images with borders using the shared web worker client.
It encapsulates all state related to the generation process.
This hook must be used within a component tree wrapped by `<WorkerProvider />`.

## Returns

[`UseGenerateBordersReturn`](../interfaces/UseGenerateBordersReturn.md)

Hook state and actions for border generation.

## Author

Edwin Zhan

## Since

1.1.0
