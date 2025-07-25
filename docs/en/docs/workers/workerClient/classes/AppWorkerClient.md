[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / AppWorkerClient

# Class: AppWorkerClient

Defined in: [workers/workerClient.ts:9](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L9)

A unified client to manage and interact with all web workers in the application.
This class provides a clean, promise-based API for computationally expensive tasks,
abstracting away the underlying `postMessage` communication.

## Author

Edwin Zhan

## Since

1.1.0

## Constructors

### Constructor

> **new AppWorkerClient**(): `AppWorkerClient`

Defined in: [workers/workerClient.ts:18](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L18)

#### Returns

`AppWorkerClient`

## Methods

### abortExportImage()

> **abortExportImage**(): `void`

Defined in: [workers/workerClient.ts:232](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L232)

#### Returns

`void`

***

### abortExtractExif()

> **abortExtractExif**(): `void`

Defined in: [workers/workerClient.ts:271](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L271)

#### Returns

`void`

***

### abortGenerateBorders()

> **abortGenerateBorders**(): `void`

Defined in: [workers/workerClient.ts:183](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L183)

#### Returns

`void`

***

### abortGenerateHash()

> **abortGenerateHash**(): `void`

Defined in: [workers/workerClient.ts:144](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L144)

#### Returns

`void`

***

### abortGenerateThumbnail()

> **abortGenerateThumbnail**(): `void`

Defined in: [workers/workerClient.ts:103](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L103)

#### Returns

`void`

***

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:48](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L48)

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

### exportImage()

> **exportImage**(`imageUrl`, `options`): `Promise`\<\{ `blob?`: `Blob`; `error?`: `string`; `filename?`: `string`; `status`: `"error"` \| `"complete"`; \}\>

Defined in: [workers/workerClient.ts:188](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L188)

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

Defined in: [workers/workerClient.ts:237](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L237)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### generateBorders()

> **generateBorders**(`files`, `option`, `param`): `Promise`\<\{[`uuid`: `string`]: `object`; \}\>

Defined in: [workers/workerClient.ts:149](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L149)

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

Defined in: [workers/workerClient.ts:108](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L108)

#### Parameters

##### data

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

***

### generateThumbnail()

> **generateThumbnail**(`data`): `Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:59](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L59)

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

### terminateAllWorkers()

> **terminateAllWorkers**(): `void`

Defined in: [workers/workerClient.ts:280](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/99610cb9c602f64ece6706d76967bc3cfa8eaab9/web/src/workers/workerClient.ts#L280)

Terminates all active workers to clean up resources.
This should be called when the application is unmounting.

#### Returns

`void`
