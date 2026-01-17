[lumilio-web](../../../modules.md) / [contexts/WorkerProvider](../index.md) / useWorker

# Function: useWorker()

> **useWorker**(): [`AppWorkerClient`](../../../workers/workerClient/classes/AppWorkerClient.md)

Defined in: [contexts/WorkerProvider.tsx:14](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/700e54a9fc9657147393b731855c580bfadc21f3/web/src/contexts/WorkerProvider.tsx#L14)

Custom hook to safely access the AppWorkerClient instance from the context.
It ensures that the hook is used within a component wrapped by WorkerProvider.

## Returns

[`AppWorkerClient`](../../../workers/workerClient/classes/AppWorkerClient.md)

The shared instance of the worker client.

## Throws

If the hook is used outside of a WorkerProvider.
