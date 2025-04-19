[**Documentation**](../../../README.md)

***

[Documentation](../../../README.md) / [wasm-hooks/useGenerateThumbnail](../README.md) / useGenerateThumbnail

# Function: useGenerateThumbnail()

> **useGenerateThumbnail**(`options`): `object`

Defined in: [wasm-hooks/useGenerateThumbnail.tsx:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03b27b5b17ee0a42274724b4bec37a2dbd4ae9d2/web/src/hooks/wasm-hooks/useGenerateThumbnail.tsx#L28)

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
