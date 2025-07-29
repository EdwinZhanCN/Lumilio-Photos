[lumilio-web](../../../modules.md) / [contexts/WorkerProvider](../index.md) / useWorker

# Function: useWorker()

> **useWorker**(): [`AppWorkerClient`](../../../workers/workerClient/classes/AppWorkerClient.md)

Defined in: [contexts/WorkerProvider.tsx:13](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/0cb9b6c9a2e1869ca5ea4411f957d39edc719928/web/src/contexts/WorkerProvider.tsx#L13)

Custom hook to safely access the AppWorkerClient instance from the context.
It ensures that the hook is used within a component wrapped by WorkerProvider.

## Returns

[`AppWorkerClient`](../../../workers/workerClient/classes/AppWorkerClient.md)

The shared instance of the worker client.

## Throws

If the hook is used outside of a WorkerProvider.
