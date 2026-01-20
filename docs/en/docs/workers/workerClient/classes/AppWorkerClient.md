[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / AppWorkerClient

# Class: AppWorkerClient

Defined in: [workers/workerClient.ts:16](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L16)

## Constructors

### Constructor

> **new AppWorkerClient**(`options`): `AppWorkerClient`

Defined in: [workers/workerClient.ts:25](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L25)

#### Parameters

##### options

[`WorkerClientOptions`](../interfaces/WorkerClientOptions.md) = `{}`

#### Returns

`AppWorkerClient`

## Methods

### abortExportImage()

> **abortExportImage**(): `void`

Defined in: [workers/workerClient.ts:291](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L291)

#### Returns

`void`

***

### abortExtractExif()

> **abortExtractExif**(): `void`

Defined in: [workers/workerClient.ts:334](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L334)

#### Returns

`void`

***

### abortGenerateBorders()

> **abortGenerateBorders**(): `void`

Defined in: [workers/workerClient.ts:238](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L238)

#### Returns

`void`

***

### abortGenerateHash()

> **abortGenerateHash**(): `void`

Defined in: [workers/workerClient.ts:193](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L193)

#### Returns

`void`

***

### abortGenerateThumbnail()

> **abortGenerateThumbnail**(): `void`

Defined in: [workers/workerClient.ts:148](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L148)

#### Returns

`void`

***

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:96](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L96)

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

Defined in: [workers/workerClient.ts:245](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L245)

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

Defined in: [workers/workerClient.ts:298](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L298)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### generateBorders()

> **generateBorders**(`files`, `option`, `param`): `Promise`\<\{\[`uuid`: `string`\]: `object`; \}\>

Defined in: [workers/workerClient.ts:200](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L200)

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

Defined in: [workers/workerClient.ts:155](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L155)

#### Parameters

##### data

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `hashResults`: `object`[]; `status`: `string`; \}\>

***

### generateThumbnail()

> **generateThumbnail**(`data`): `Promise`\<\{ `batchIndex`: `number`; `results`: `any`[]; `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:107](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L107)

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

Defined in: [workers/workerClient.ts:345](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/e359d4441f14ec80c9eda9ee68ada26fd7bbb8e0/web/src/workers/workerClient.ts#L345)

Terminates all active workers to clean up resources.
This should be called when the application is unmounting.

#### Returns

`void`
