[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateHashcode](../index.md) / useGenerateHashcode

# Function: useGenerateHashcode()

> **useGenerateHashcode**(`onPerformanceMetrics?`): [`useGenerateHashcodeReturn`](../interfaces/useGenerateHashcodeReturn.md)

Defined in: hooks/util-hooks/useGenerateHashcode.tsx:43

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
