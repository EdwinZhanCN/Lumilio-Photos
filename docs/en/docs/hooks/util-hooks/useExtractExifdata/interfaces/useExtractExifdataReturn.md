[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useExtractExifdata](../index.md) / useExtractExifdataReturn

# Interface: useExtractExifdataReturn

Defined in: [hooks/util-hooks/useExtractExifdata.tsx:18](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/001ab38fa17e95bfd94631c19af859c7e7a3185a/web/src/hooks/util-hooks/useExtractExifdata.tsx#L18)

## Properties

### cancelExtraction()

> **cancelExtraction**: () => `void`

Defined in: [hooks/util-hooks/useExtractExifdata.tsx:23](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/001ab38fa17e95bfd94631c19af859c7e7a3185a/web/src/hooks/util-hooks/useExtractExifdata.tsx#L23)

#### Returns

`void`

***

### exifData

> **exifData**: `null` \| `Record`\<`number`, `any`\>

Defined in: [hooks/util-hooks/useExtractExifdata.tsx:20](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/001ab38fa17e95bfd94631c19af859c7e7a3185a/web/src/hooks/util-hooks/useExtractExifdata.tsx#L20)

***

### extractExifData()

> **extractExifData**: (`files`, `priority?`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useExtractExifdata.tsx:22](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/001ab38fa17e95bfd94631c19af859c7e7a3185a/web/src/hooks/util-hooks/useExtractExifdata.tsx#L22)

#### Parameters

##### files

`File`[]

##### priority?

[`ProcessingPriority`](../../../../utils/smartBatchSizing/enumerations/ProcessingPriority.md)

#### Returns

`Promise`\<`void`\>

***

### isExtracting

> **isExtracting**: `boolean`

Defined in: [hooks/util-hooks/useExtractExifdata.tsx:19](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/001ab38fa17e95bfd94631c19af859c7e7a3185a/web/src/hooks/util-hooks/useExtractExifdata.tsx#L19)

***

### progress

> **progress**: [`ExifExtractionProgress`](../type-aliases/ExifExtractionProgress.md)

Defined in: [hooks/util-hooks/useExtractExifdata.tsx:21](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/001ab38fa17e95bfd94631c19af859c7e7a3185a/web/src/hooks/util-hooks/useExtractExifdata.tsx#L21)
