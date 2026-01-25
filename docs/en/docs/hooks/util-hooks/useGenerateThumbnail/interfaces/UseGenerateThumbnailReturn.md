[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateThumbnail](../index.md) / UseGenerateThumbnailReturn

# Interface: UseGenerateThumbnailReturn

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:25](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L25)

Represents the state and actions returned by the useGenerateThumbnail hook.

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:29](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L29)

#### Returns

`void`

***

### generatePreviews()

> **generatePreviews**: (`files`, `priority?`) => `Promise`\<`ThumbnailResult`[] \| `undefined`\>

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L28)

#### Parameters

##### files

`File`[]

##### priority?

[`ProcessingPriority`](../../../../utils/smartBatchSizing/enumerations/ProcessingPriority.md)

#### Returns

`Promise`\<`ThumbnailResult`[] \| `undefined`\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L26)

***

### progress

> **progress**: [`ThumbnailProgress`](ThumbnailProgress.md) \| `null`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:27](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/688e6b882d759a1db5e0f0c3da3d0cd075f00e23/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L27)
