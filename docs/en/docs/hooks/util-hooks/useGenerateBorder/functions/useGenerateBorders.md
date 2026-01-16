[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateBorder](../index.md) / useGenerateBorders

# Function: useGenerateBorders()

> **useGenerateBorders**(): [`UseGenerateBordersReturn`](../interfaces/UseGenerateBordersReturn.md)

Defined in: [hooks/util-hooks/useGenerateBorder.tsx:71](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/001ab38fa17e95bfd94631c19af859c7e7a3185a/web/src/hooks/util-hooks/useGenerateBorder.tsx#L71)

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
