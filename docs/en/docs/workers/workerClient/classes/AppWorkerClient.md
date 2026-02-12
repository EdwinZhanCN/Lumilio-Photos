[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / AppWorkerClient

# Class: AppWorkerClient

Defined in: [workers/workerClient.ts:36](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L36)

## Constructors

### Constructor

> **new AppWorkerClient**(`options?`): `AppWorkerClient`

Defined in: [workers/workerClient.ts:48](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L48)

#### Parameters

##### options?

[`WorkerClientOptions`](../interfaces/WorkerClientOptions.md) = `{}`

#### Returns

`AppWorkerClient`

## Methods

### abortExportImage()

> **abortExportImage**(): `void`

Defined in: [workers/workerClient.ts:449](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L449)

#### Returns

`void`

***

### abortExtractExif()

> **abortExtractExif**(): `void`

Defined in: [workers/workerClient.ts:492](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L492)

#### Returns

`void`

***

### abortGenerateBorders()

> **abortGenerateBorders**(): `void`

Defined in: [workers/workerClient.ts:396](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L396)

#### Returns

`void`

***

### abortGenerateHash()

> **abortGenerateHash**(): `void`

Defined in: [workers/workerClient.ts:355](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L355)

#### Returns

`void`

***

### abortGenerateThumbnail()

> **abortGenerateThumbnail**(): `void`

Defined in: [workers/workerClient.ts:272](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L272)

#### Returns

`void`

***

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:128](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L128)

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

### calculateJustifiedLayout()

> **calculateJustifiedLayout**(`boxes`, `config`): `Promise`\<`LayoutResult`\>

Defined in: [workers/workerClient.ts:170](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L170)

#### Parameters

##### boxes

`LayoutBox`[]

##### config

`LayoutConfig`

#### Returns

`Promise`\<`LayoutResult`\>

***

### calculateJustifiedLayouts()

> **calculateJustifiedLayouts**(`groups`, `config`): `Promise`\<`Record`\<`string`, `LayoutResult`\>\>

Defined in: [workers/workerClient.ts:200](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L200)

#### Parameters

##### groups

`Record`\<`string`, `LayoutBox`[]\>

##### config

`LayoutConfig`

#### Returns

`Promise`\<`Record`\<`string`, `LayoutResult`\>\>

***

### exportImage()

> **exportImage**(`imageUrl`, `options`): `Promise`\<\{ `blob?`: `Blob`; `error?`: `string`; `filename?`: `string`; `status`: `"error"` \| `"complete"`; \}\>

Defined in: [workers/workerClient.ts:403](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L403)

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

Defined in: [workers/workerClient.ts:456](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L456)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### generateBorders()

> **generateBorders**(`files`, `option`, `param`): `Promise`\<\{\[`uuid`: `string`\]: `object`; \}\>

Defined in: [workers/workerClient.ts:360](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L360)

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

Defined in: [workers/workerClient.ts:279](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L279)

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

Defined in: [workers/workerClient.ts:231](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L231)

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

### initializeJustifiedLayout()

> **initializeJustifiedLayout**(): `Promise`\<`void`\>

Defined in: [workers/workerClient.ts:143](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L143)

#### Returns

`Promise`\<`void`\>

***

### terminateAllWorkers()

> **terminateAllWorkers**(): `void`

Defined in: [workers/workerClient.ts:503](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87550f699d3d5501c5ee53bfa6a214c4b59cf82a/web/src/workers/workerClient.ts#L503)

Terminates all active workers to clean up resources.
This should be called when the application is unmounting.

#### Returns

`void`
