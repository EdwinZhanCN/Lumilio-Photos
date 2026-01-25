[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / AppWorkerClient

# Class: AppWorkerClient

Defined in: [workers/workerClient.ts:25](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L25)

## Constructors

### Constructor

> **new AppWorkerClient**(`options`): `AppWorkerClient`

Defined in: [workers/workerClient.ts:34](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L34)

#### Parameters

##### options

[`WorkerClientOptions`](../interfaces/WorkerClientOptions.md) = `{}`

#### Returns

`AppWorkerClient`

## Methods

### abortExportImage()

> **abortExportImage**(): `void`

Defined in: [workers/workerClient.ts:334](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L334)

#### Returns

`void`

***

### abortExtractExif()

> **abortExtractExif**(): `void`

Defined in: [workers/workerClient.ts:377](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L377)

#### Returns

`void`

***

### abortGenerateBorders()

> **abortGenerateBorders**(): `void`

Defined in: [workers/workerClient.ts:281](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L281)

#### Returns

`void`

***

### abortGenerateHash()

> **abortGenerateHash**(): `void`

Defined in: [workers/workerClient.ts:240](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L240)

#### Returns

`void`

***

### abortGenerateThumbnail()

> **abortGenerateThumbnail**(): `void`

Defined in: [workers/workerClient.ts:157](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L157)

#### Returns

`void`

***

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:105](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L105)

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

Defined in: [workers/workerClient.ts:288](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L288)

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

Defined in: [workers/workerClient.ts:341](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L341)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### generateBorders()

> **generateBorders**(`files`, `option`, `param`): `Promise`\<\{\[`uuid`: `string`\]: `object`; \}\>

Defined in: [workers/workerClient.ts:245](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L245)

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

> **generateHash**(`data`, `onItemComplete?`): `Promise`\<\{ `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:164](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L164)

#### Parameters

##### data

`FileList` | `File`[]

##### onItemComplete?

(`result`) => `void`

#### Returns

`Promise`\<\{ `status`: `string`; \}\>

***

### generateThumbnail()

> **generateThumbnail**(`data`): `Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:116](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L116)

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

Defined in: [workers/workerClient.ts:388](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/workers/workerClient.ts#L388)

Terminates all active workers to clean up resources.
This should be called when the application is unmounting.

#### Returns

`void`
