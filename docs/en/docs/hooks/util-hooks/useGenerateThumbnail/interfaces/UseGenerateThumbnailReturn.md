[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useGenerateThumbnail](../index.md) / UseGenerateThumbnailReturn

# Interface: UseGenerateThumbnailReturn

Defined in: hooks/util-hooks/useGenerateThumbnail.tsx:22

Represents the state and actions returned by the useGenerateThumbnail hook.

## Properties

### cancelGeneration()

> **cancelGeneration**: () => `void`

Defined in: hooks/util-hooks/useGenerateThumbnail.tsx:26

#### Returns

`void`

***

### generatePreviews()

> **generatePreviews**: (`files`) => `Promise`\<`undefined` \| `ThumbnailResult`[]\>

Defined in: hooks/util-hooks/useGenerateThumbnail.tsx:25

#### Parameters

##### files

`File`[]

#### Returns

`Promise`\<`undefined` \| `ThumbnailResult`[]\>

***

### isGenerating

> **isGenerating**: `boolean`

Defined in: hooks/util-hooks/useGenerateThumbnail.tsx:23

***

### progress

> **progress**: `null` \| [`ThumbnailProgress`](ThumbnailProgress.md)

Defined in: hooks/util-hooks/useGenerateThumbnail.tsx:24
