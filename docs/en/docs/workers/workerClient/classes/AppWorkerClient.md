[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / AppWorkerClient

# Class: AppWorkerClient

Defined in: [workers/workerClient.ts:69](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L69)

## Constructors

### Constructor

> **new AppWorkerClient**(`options`): `AppWorkerClient`

Defined in: [workers/workerClient.ts:86](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L86)

#### Parameters

##### options

[`WorkerClientOptions`](../interfaces/WorkerClientOptions.md) = `{}`

#### Returns

`AppWorkerClient`

## Methods

### abortExportImage()

> **abortExportImage**(): `void`

Defined in: [workers/workerClient.ts:369](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L369)

#### Returns

`void`

***

### abortExtractExif()

> **abortExtractExif**(): `void`

Defined in: [workers/workerClient.ts:412](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L412)

#### Returns

`void`

***

### abortGenerateBorders()

> **abortGenerateBorders**(): `void`

Defined in: [workers/workerClient.ts:316](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L316)

#### Returns

`void`

***

### abortGenerateHash()

> **abortGenerateHash**(): `void`

Defined in: [workers/workerClient.ts:271](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L271)

#### Returns

`void`

***

### abortGenerateThumbnail()

> **abortGenerateThumbnail**(): `void`

Defined in: [workers/workerClient.ts:226](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L226)

#### Returns

`void`

***

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:175](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L175)

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

Defined in: [workers/workerClient.ts:460](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L460)

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

Defined in: [workers/workerClient.ts:323](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L323)

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

Defined in: [workers/workerClient.ts:376](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L376)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### generateBorders()

> **generateBorders**(`files`, `option`, `param`): `Promise`\<\{[`uuid`: `string`]: `object`; \}\>

Defined in: [workers/workerClient.ts:278](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L278)

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

Defined in: [workers/workerClient.ts:233](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L233)

#### Parameters

##### data

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

***

### generateThumbnail()

> **generateThumbnail**(`data`): `Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:186](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L186)

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

Defined in: [workers/workerClient.ts:421](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L421)

Initialize the WebLLM engine that communicates with the worker

#### Parameters

##### modelId

`string` = `...`

#### Returns

`Promise`\<`void`\>

***

### terminateAllWorkers()

> **terminateAllWorkers**(): `void`

Defined in: [workers/workerClient.ts:517](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03970823ed92f529d8017eeae43ca1cadd7110c3/web/src/workers/workerClient.ts#L517)

Terminates all active workers to clean up resources.
This should be called when the application is unmounting.

#### Returns

`void`
