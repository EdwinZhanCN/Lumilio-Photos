[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateThumbnail](../index.md) / UseGenerateThumbnailReturn

# Interface: UseGenerateThumbnailReturn

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:22](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/5a9be158f2088be7556fada16832ccc8d88ac157/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L22)

Represents the state and actions returned by the useGenerateThumbnail hook.

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/5a9be158f2088be7556fada16832ccc8d88ac157/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L26)

#### Returns

`void`

***

### generatePreviews()

> **generatePreviews**: (`files`) => `Promise`\<`undefined` \| `ThumbnailResult`[]\>

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:25](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/5a9be158f2088be7556fada16832ccc8d88ac157/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L25)

#### Parameters

##### files

`File`[]

#### Returns

`Promise`\<`undefined` \| `ThumbnailResult`[]\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:23](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/5a9be158f2088be7556fada16832ccc8d88ac157/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L23)

***

### progress

> **progress**: `null` \| [`ThumbnailProgress`](ThumbnailProgress.md)

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:24](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/5a9be158f2088be7556fada16832ccc8d88ac157/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L24)
