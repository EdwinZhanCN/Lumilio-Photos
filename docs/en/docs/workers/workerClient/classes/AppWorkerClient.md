[lumilio-web](../../../modules.md) / [workers/workerClient](../index.md) / AppWorkerClient

# Class: AppWorkerClient

Defined in: [workers/workerClient.ts:37](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L37)

## Constructors

### Constructor

> **new AppWorkerClient**(`options?`): `AppWorkerClient`

Defined in: [workers/workerClient.ts:51](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L51)

#### Parameters

##### options?

[`WorkerClientOptions`](../interfaces/WorkerClientOptions.md) = `{}`

#### Returns

`AppWorkerClient`

## Methods

### abortExportImage()

> **abortExportImage**(): `void`

Defined in: [workers/workerClient.ts:554](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L554)

#### Returns

`void`

***

### abortExtractExif()

> **abortExtractExif**(): `void`

Defined in: [workers/workerClient.ts:597](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L597)

#### Returns

`void`

***

### abortGenerateHash()

> **abortGenerateHash**(): `void`

Defined in: [workers/workerClient.ts:367](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L367)

#### Returns

`void`

***

### abortGenerateThumbnail()

> **abortGenerateThumbnail**(): `void`

Defined in: [workers/workerClient.ts:284](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L284)

#### Returns

`void`

***

### abortStudioPlugin()

> **abortStudioPlugin**(): `void`

Defined in: [workers/workerClient.ts:501](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L501)

#### Returns

`void`

***

### addProgressListener()

> **addProgressListener**(`callback`): () => `void`

Defined in: [workers/workerClient.ts:131](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L131)

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

Defined in: [workers/workerClient.ts:182](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L182)

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

Defined in: [workers/workerClient.ts:212](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L212)

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

Defined in: [workers/workerClient.ts:508](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L508)

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

Defined in: [workers/workerClient.ts:561](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L561)

#### Parameters

##### files

`FileList` | `File`[]

#### Returns

`Promise`\<\{ `exifResults`: `object`[]; `status`: `string`; \}\>

***

### generateHash()

> **generateHash**(`data`, `onItemComplete?`): `Promise`\<\{ `status`: `string`; \}\>

Defined in: [workers/workerClient.ts:291](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L291)

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

Defined in: [workers/workerClient.ts:243](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L243)

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

Defined in: [workers/workerClient.ts:155](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L155)

#### Returns

`Promise`\<`void`\>

***

### loadStudioPluginRunner()

> **loadStudioPluginRunner**(`manifest`): `Promise`\<`void`\>

Defined in: [workers/workerClient.ts:372](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L372)

#### Parameters

##### manifest

`RuntimeManifestV1`

#### Returns

`Promise`\<`void`\>

***

### runStudioPlugin()

> **runStudioPlugin**(`manifest`, `file`, `params`): `Promise`\<\{ `blob`: `Blob`; `fileName`: `string`; `mimeType`: `string`; \}\>

Defined in: [workers/workerClient.ts:425](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L425)

#### Parameters

##### manifest

`RuntimeManifestV1`

##### file

`File`

##### params

`Record`\<`string`, `unknown`\>

#### Returns

`Promise`\<\{ `blob`: `Blob`; `fileName`: `string`; `mimeType`: `string`; \}\>

***

### terminateAllWorkers()

> **terminateAllWorkers**(): `void`

Defined in: [workers/workerClient.ts:608](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/d0b4da507ab9c0740bf81054daa3d8ad6a258681/web/src/workers/workerClient.ts#L608)

Terminates all active workers to clean up resources.
This should be called when the application is unmounting.

#### Returns

`void`
