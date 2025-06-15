[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / WasmWorkerClient

# Class: WasmWorkerClient

Defined in: [workers/workerClient.ts:5](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L5)

This class is a wrapper around the Web Worker API to facilitate communication
with a WebAssembly worker.

## Constructors

### Constructor

> **new WasmWorkerClient**(): `WasmWorkerClient`

Defined in: [workers/workerClient.ts:13](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L13)

Creates an instance of WasmWorkerClient.

#### Returns

`WasmWorkerClient`

## Methods

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L28)

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

### generateHash()

> **generateHash**(`data`): `Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:178](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L178)

Processes files into hashcodes, sending the results hash back to the main thread.
You may want to use catch to handle the error.

#### Parameters

##### data

`FileList`

The data to be processed. List of files.

#### Returns

`Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

#### Requires

FileList

***

### generateThumbnail()

> **generateThumbnail**(`data`): `Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:70](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L70)

Processes files in batches, sending the results thumbnails back to the main thread.
You may want to use catch to handle the error.-

#### Parameters

##### data

The data to be processed. List of files, batch index, and start index.

###### batchIndex

`number`

###### files

`FileList` \| `File`[]

###### startIndex

`number`

#### Returns

`Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

***

### initGenHashWASM()

> **initGenHashWASM**(`timeoutMs`): `Promise`\<\{ `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:146](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L146)

This function is used to initialize the WebAssembly module in genHash worker script.

#### Parameters

##### timeoutMs

`number` = `100000`

Timeout in milliseconds

#### Returns

`Promise`\<\{ `status`: `string`; \}\>

***

### initGenThumbnailWASM()

> **initGenThumbnailWASM**(`timeoutMs`): `Promise`\<\{ `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:39](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L39)

Initializes the WebAssembly module in genThumbnail worker script.

#### Parameters

##### timeoutMs

`number` = `100000`

Timeout in milliseconds

#### Returns

`Promise`\<\{ `status`: `string`; \}\>

***

### terminateGenerateHashWorker()

> **terminateGenerateHashWorker**(): `void`

Defined in: [workers/workerClient.ts:255](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L255)

Terminates the genHash worker.

#### Returns

`void`

***

### terminateGenerateThumbnailWorker()

> **terminateGenerateThumbnailWorker**(): `void`

Defined in: [workers/workerClient.ts:248](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/workers/workerClient.ts#L248)

Terminates the genThumbnail worker.

#### Returns

`void`
