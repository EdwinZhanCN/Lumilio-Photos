[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcode

# Function: useGenerateHashcode()

> **useGenerateHashcode**(`onPerformanceMetrics?`): [`useGenerateHashcodeReturn`](../interfaces/useGenerateHashcodeReturn.md)

Defined in: [hooks/util-hooks/useGenerateHashcode.tsx:49](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/67c9aa6e9757c27514a72920fc1c7730f5f2ba78/web/src/hooks/util-hooks/useGenerateHashcode.tsx#L49)

Custom hook for generating file hashcodes using a Web Worker.
Manages its own state for progress and generation status.
This hook must be used within a component tree wrapped by `<WorkerProvider />`.

## Parameters

### onPerformanceMetrics?

(`metrics`) => `void`

## Returns

[`useGenerateHashcodeReturn`](../interfaces/useGenerateHashcodeReturn.md)

Hook state and actions for hashcode generation.

## Author

Edwin Zhan

## Since

1.1.0
