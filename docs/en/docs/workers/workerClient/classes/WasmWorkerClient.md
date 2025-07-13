[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / WasmWorkerClient

# Class: WasmWorkerClient

Defined in: [workers/workerClient.ts:5](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L5)

This class is a wrapper around the Web Worker API to facilitate communication
with a WebAssembly worker.

## Constructors

### Constructor

> **new WasmWorkerClient**(): `WasmWorkerClient`

Defined in: [workers/workerClient.ts:14](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L14)

Creates an instance of WasmWorkerClient.

#### Returns

`WasmWorkerClient`

## Methods

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:41](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L41)

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

### generateBorders()

> **generateBorders**(`files`, `option`, `param`): `Promise`\<\{[`uuid`: `string`]: `object`; \}\>

Defined in: [workers/workerClient.ts:339](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L339)

Processes a list of files to add borders using the specified options and parameters.

#### Parameters

##### files

`File`[]

The list of files to process.

##### option

The type of border to apply.

`"COLORED"` | `"FROSTED"` | `"VIGNETTE"`

##### param

`object`

The parameters for the selected border type.

#### Returns

`Promise`\<\{[`uuid`: `string`]: `object`; \}\>

A promise that resolves with an object mapping UUIDs to processing results.

***

### generateHash()

> **generateHash**(`data`): `Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:225](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L225)

Processes files into hashcodes, sending the results hash back to the main thread.
You may want to use catch to handle the error.

#### Parameters

##### data

The data to be processed. List of files.

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

#### Requires

FileList | File[] - The data to be processed. List of files.

***

### generateThumbnail()

> **generateThumbnail**(`data`): `Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:93](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L93)

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

### initBorderWASM()

> **initBorderWASM**(`timeoutMs`): `Promise`\<\{ `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:301](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L301)

#### Parameters

##### timeoutMs

`number` = `100000`

#### Returns

`Promise`\<\{ `status`: `string`; \}\>

***

### initGenHashWASM()

> **initGenHashWASM**(`timeoutMs`): `Promise`\<\{ `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:187](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L187)

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

Defined in: [workers/workerClient.ts:56](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L56)

Initializes the WebAssembly module in genThumbnail worker script.

#### Parameters

##### timeoutMs

`number` = `100000`

Timeout in milliseconds

#### Returns

`Promise`\<\{ `status`: `string`; \}\>

***

### terminateGenerateBorderWorker()

> **terminateGenerateBorderWorker**(): `void`

Defined in: [workers/workerClient.ts:402](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L402)

Terminates the Web Worker.

#### Returns

`void`

***

### terminateGenerateHashWorker()

> **terminateGenerateHashWorker**(): `void`

Defined in: [workers/workerClient.ts:416](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L416)

Terminates the genHash worker.

#### Returns

`void`

***

### terminateGenerateThumbnailWorker()

> **terminateGenerateThumbnailWorker**(): `void`

Defined in: [workers/workerClient.ts:409](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/workers/workerClient.ts#L409)

Terminates the genThumbnail worker.

#### Returns

`void`
