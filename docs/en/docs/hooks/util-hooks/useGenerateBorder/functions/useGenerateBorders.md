[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateBorder](../index.md) / useGenerateBorders

# Function: useGenerateBorders()

> **useGenerateBorders**(): [`UseGenerateBordersReturn`](../interfaces/UseGenerateBordersReturn.md)

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:71](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/5721bfe3c3de0a6b4a87ccfecb368f9fd5719f61/web/src/hooks/util-hooks/useGenerateBorder.tsx#L71)

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
