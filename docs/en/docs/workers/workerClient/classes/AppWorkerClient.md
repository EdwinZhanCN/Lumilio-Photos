[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / AppWorkerClient

# Class: AppWorkerClient

Defined in: [workers/workerClient.ts:69](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L69)

## Constructors

### Constructor

> **new AppWorkerClient**(`options`): `AppWorkerClient`

Defined in: [workers/workerClient.ts:86](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L86)

#### Parameters

##### options

[`WorkerClientOptions`](../interfaces/WorkerClientOptions.md) = `{}`

#### Returns

`AppWorkerClient`

## Methods

### abortExportImage()

> **abortExportImage**(): `void`

Defined in: [workers/workerClient.ts:367](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L367)

#### Returns

`void`

***

### abortExtractExif()

> **abortExtractExif**(): `void`

Defined in: [workers/workerClient.ts:410](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L410)

#### Returns

`void`

***

### abortGenerateBorders()

> **abortGenerateBorders**(): `void`

Defined in: [workers/workerClient.ts:314](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L314)

#### Returns

`void`

***

### abortGenerateHash()

> **abortGenerateHash**(): `void`

Defined in: [workers/workerClient.ts:271](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L271)

#### Returns

`void`

***

### abortGenerateThumbnail()

> **abortGenerateThumbnail**(): `void`

Defined in: [workers/workerClient.ts:226](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L226)

#### Returns

`void`

***

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:175](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L175)

Adds a progress listener that can be used by any worker task.

#### Parameters

##### callback

(`detail`) => `void`

Function to handle progress events.

#### Returns

A function to remove the event listener.

> (): `void`

##### Returns

`void`

***

### askLLM()

> **askLLM**(`messages`, `options`): `Promise`\<`string`\>

Defined in: [workers/workerClient.ts:458](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L458)

Ask the LLM a question with streaming response

#### Parameters

##### messages

`ChatCompletionMessageParam`[]

##### options

###### onChunk?

(`chunk`) => `void`

###### stream?

`boolean`

###### temperature?

`number`

#### Returns

`Promise`\<`string`\>

***

### exportImage()

> **exportImage**(`imageUrl`, `options`): `Promise`\<\{ `blob?`: `Blob`; `error?`: `string`; `filename?`: `string`; `status`: `"error"` \| `"complete"`; \}\>

Defined in: [workers/workerClient.ts:321](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L321)

#### Parameters

##### imageUrl

`string`

##### options

###### filename?

`string`

###### format

`"jpeg"` \| `"png"` \| `"webp"` \| `"original"`

###### maxHeight?

`number`

###### maxWidth?

`number`

###### quality

`number`

#### Returns

`Promise`\<\{ `blob?`: `Blob`; `error?`: `string`; `filename?`: `string`; `status`: `"error"` \| `"complete"`; \}\>

***

### extractExif()

> **extractExif**(`files`): `Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:374](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L374)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### generateBorders()

> **generateBorders**(`files`, `option`, `param`): `Promise`\<\{[`uuid`: `string`]: `object`; \}\>

Defined in: [workers/workerClient.ts:278](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L278)

#### Parameters

##### files

`File`[]

##### option

`"COLORED"` | `"FROSTED"` | `"VIGNETTE"`

##### param

`object`

#### Returns

`Promise`\<\{[`uuid`: `string`]: `object`; \}\>

***

### generateHash()

> **generateHash**(`data`): `Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:233](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L233)

#### Parameters

##### data

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

***

### generateThumbnail()

> **generateThumbnail**(`data`): `Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:186](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L186)

#### Parameters

##### data

###### batchIndex

`number`

###### files

`FileList` \| `File`[]

###### startIndex

`number`

#### Returns

`Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

***

### initializeWebLLMEngine()

> **initializeWebLLMEngine**(`modelId`): `Promise`\<`void`\>

Defined in: [workers/workerClient.ts:419](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L419)

Initialize the WebLLM engine that communicates with the worker

#### Parameters

##### modelId

`string` = `...`

#### Returns

`Promise`\<`void`\>

***

### terminateAllWorkers()

> **terminateAllWorkers**(): `void`

Defined in: [workers/workerClient.ts:515](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/50447139bbcd8646ed06f83c6f5775c49db37354/web/src/workers/workerClient.ts#L515)

Terminates all active workers to clean up resources.
This should be called when the application is unmounting.

#### Returns

`void`
