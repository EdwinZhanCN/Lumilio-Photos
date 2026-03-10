[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcode

# Function: useGenerateHashcode()

> **useGenerateHashcode**(): [`useGenerateHashcodeReturn`](../interfaces/useGenerateHashcodeReturn.md)

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:35](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/c61b22df54ea7cb37ab4a4bd7817fe9e39968a81/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L35)

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
