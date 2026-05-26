[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcode

# Function: useGenerateHashcode()

> **useGenerateHashcode**(): [`useGenerateHashcodeReturn`](../interfaces/useGenerateHashcodeReturn.md)

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:35](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3823d466f37d5d6dcd67b116921540ea5db7a8f6/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L35)

Custom hook for generating file hashcodes using a Web Worker.
Manages its own state for progress and generation status.
This hook must be used within a component tree wrapped by `<WorkerProvider />`.

## Returns

[`useGenerateHashcodeReturn`](../interfaces/useGenerateHashcodeReturn.md)

Hook state and actions for hashcode generation.

## Author

Edwin Zhan

## Since

1.1.0
