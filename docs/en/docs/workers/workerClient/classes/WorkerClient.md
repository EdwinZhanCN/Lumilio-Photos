[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / WorkerClient

# Class: WorkerClient

Defined in: [workers/workerClient.ts:421](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L421)

## Constructors

### Constructor

> **new WorkerClient**(): `WorkerClient`

Defined in: [workers/workerClient.ts:425](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L425)

#### Returns

`WorkerClient`

## Methods

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:440](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L440)

Adds a progress listener to the worker.

#### Parameters

##### callback

(`detail`) => `void`

Function to handle progress events

#### Returns

- Function to remove the event listener

> (): `void`

##### Returns

`void`

***

### extractExif()

> **extractExif**(`files`): `Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:450](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L450)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### terminateExtractExifWorker()

> **terminateExtractExifWorker**(): `void`

Defined in: [workers/workerClient.ts:525](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L525)

Terminates the extractExif worker.

#### Returns

`void`
