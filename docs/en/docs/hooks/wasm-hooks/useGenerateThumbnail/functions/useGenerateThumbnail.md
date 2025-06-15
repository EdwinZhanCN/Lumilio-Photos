[lumilio-web](../../../../modules.md) / [hooks/wasm-hooks/useGenerateThumbnail](../index.md) / useGenerateThumbnail

# Function: useGenerateThumbnail()

> **useGenerateThumbnail**(`options`): `object`

Defined in: [hooks/wasm-hooks/useGenerateThumbnail.tsx:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/hooks/wasm-hooks/useGenerateThumbnail.tsx#L28)

Custom hook to generate thumbnails using a Web Worker.

## Parameters

### options

`UseGenerateThumbnailProps`

Configuration options

## Returns

`object`

Object containing the generatePreviews function

### generatePreviews()

> **generatePreviews**: (`files`) => `Promise`\<`void` \| `Error` \| `object`[]\>

#### Parameters

##### files

`File`[]

#### Returns

`Promise`\<`void` \| `Error` \| `object`[]\>

## Author

Edwin Zhan
