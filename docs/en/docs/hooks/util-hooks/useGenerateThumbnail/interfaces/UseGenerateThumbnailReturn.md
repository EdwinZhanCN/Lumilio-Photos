[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateThumbnail](../index.md) / UseGenerateThumbnailReturn

# Interface: UseGenerateThumbnailReturn

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:25](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/ca75377bce4e204cc757dc6c0c5454349e2c428c/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L25)

Represents the state and actions returned by the useGenerateThumbnail hook.

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:29](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/ca75377bce4e204cc757dc6c0c5454349e2c428c/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L29)

#### Returns

`void`

***

### generatePreviews()

> **generatePreviews**: (`files`, `priority?`) => `Promise`\<`ThumbnailResult`[] \| `undefined`\>

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/ca75377bce4e204cc757dc6c0c5454349e2c428c/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L28)

#### Parameters

##### files

`File`[]

##### priority?

`ProcessingPriority`

#### Returns

`Promise`\<`ThumbnailResult`[] \| `undefined`\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/ca75377bce4e204cc757dc6c0c5454349e2c428c/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L26)

***

### progress

> **progress**: [`ThumbnailProgress`](ThumbnailProgress.md) \| `null`

Defined in: [hooks/util-hooks/useGenerateThumbnail.tsx:27](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/ca75377bce4e204cc757dc6c0c5454349e2c428c/web/src/hooks/util-hooks/useGenerateThumbnail.tsx#L27)
