[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / AppWorkerClient

# Class: AppWorkerClient

Defined in: [workers/workerClient.ts:37](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L37)

## Constructors

### Constructor

> **new AppWorkerClient**(`options`): `AppWorkerClient`

Defined in: [workers/workerClient.ts:54](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L54)

#### Parameters

##### options

[`WorkerClientOptions`](../interfaces/WorkerClientOptions.md) = `{}`

#### Returns

`AppWorkerClient`

## Methods

### abortExportImage()

> **abortExportImage**(): `void`

Defined in: [workers/workerClient.ts:348](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L348)

#### Returns

`void`

***

### abortExtractExif()

> **abortExtractExif**(): `void`

Defined in: [workers/workerClient.ts:391](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L391)

#### Returns

`void`

***

### abortGenerateBorders()

> **abortGenerateBorders**(): `void`

Defined in: [workers/workerClient.ts:295](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L295)

#### Returns

`void`

***

### abortGenerateHash()

> **abortGenerateHash**(): `void`

Defined in: [workers/workerClient.ts:250](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L250)

#### Returns

`void`

***

### abortGenerateThumbnail()

> **abortGenerateThumbnail**(): `void`

Defined in: [workers/workerClient.ts:205](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L205)

#### Returns

`void`

***

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:153](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L153)

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

Defined in: [workers/workerClient.ts:466](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L466)

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

Defined in: [workers/workerClient.ts:302](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L302)

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

Defined in: [workers/workerClient.ts:355](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L355)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### generateBorders()

> **generateBorders**(`files`, `option`, `param`): `Promise`\<\{\[`uuid`: `string`\]: `object`; \}\>

Defined in: [workers/workerClient.ts:257](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L257)

#### Parameters

##### files

`File`[]

##### option

`"COLORED"` | `"FROSTED"` | `"VIGNETTE"`

##### param

`object`

#### Returns

`Promise`\<\{\[`uuid`: `string`\]: `object`; \}\>

***

### generateHash()

> **generateHash**(`data`): `Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:212](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L212)

#### Parameters

##### data

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

***

### generateThumbnail()

> **generateThumbnail**(`data`): `Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:164](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L164)

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

Defined in: [workers/workerClient.ts:400](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L400)

Initialize the WebLLM engine that communicates with the worker

#### Parameters

##### modelId

`string` = `...`

#### Returns

`Promise`\<`void`\>

***

### terminateAllWorkers()

> **terminateAllWorkers**(): `void`

Defined in: [workers/workerClient.ts:523](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/432b0417d593dbe534fb8fb1fc1703513592423a/web/src/workers/workerClient.ts#L523)

Terminates all active workers to clean up resources.
This should be called when the application is unmounting.

#### Returns

`void`
