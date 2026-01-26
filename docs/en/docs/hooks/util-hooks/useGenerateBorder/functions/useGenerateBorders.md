[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateBorder](../index.md) / useGenerateBorders

# Function: useGenerateBorders()

> **useGenerateBorders**(): [`UseGenerateBordersReturn`](../interfaces/UseGenerateBordersReturn.md)

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:71](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/c1ade2cae0cd52d3d07c8db26e98e243f7e665c1/web/src/hooks/util-hooks/useGenerateBorder.tsx#L71)

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
