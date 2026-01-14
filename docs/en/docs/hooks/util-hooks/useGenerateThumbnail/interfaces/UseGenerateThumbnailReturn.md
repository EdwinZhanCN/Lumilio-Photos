[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateThumbnail](../index.md) / UseGenerateThumbnailReturn

# Interface: UseGenerateThumbnailReturn

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:25](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L25)

Represents the state and actions returned by the useGenerateThumbnail hook.

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:29](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L29)

#### Returns

`void`

***

### generatePreviews()

> **generatePreviews**: (`files`, `priority?`) => `Promise`\<`undefined` \| `ThumbnailResult`[]\>

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L28)

#### Parameters

##### files

`File`[]

##### priority?

[`ProcessingPriority`](../../../../utils/smartBatchSizing/enumerations/ProcessingPriority.md)

#### Returns

`Promise`\<`undefined` \| `ThumbnailResult`[]\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L26)

***

### progress

> **progress**: `null` \| [`ThumbnailProgress`](ThumbnailProgress.md)

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:27](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/63d906ff8915a62b27825a8beb4fcfc311301987/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L27)
