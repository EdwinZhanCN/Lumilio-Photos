[**Documentation**](../../../README.md)

***

[Documentation](../../../README.md) / [api-hooks/useUploadProcess](../README.md) / useUploadProcess

# Function: useUploadProcess()

> **useUploadProcess**(`workerClientRef`, `wasmReady`): `object`

Defined in: [api-hooks/useUploadProcess.tsx:49](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/03b27b5b17ee0a42274724b4bec37a2dbd4ae9d2/web/src/hooks/api-hooks/useUploadProcess.tsx#L49)

useUploadProcess is a custom hook that handles the upload process of files.

## Parameters

### workerClientRef

`RefObject`\<`any`\>

Reference to your WASM worker client

### wasmReady

`boolean`

Indicates if WASM is ready

## Returns

`object`

### hashcodeProgress

> **hashcodeProgress**: `null` \| \{ `error`: `string`; `failedAt`: `number`; `numberProcessed`: `number`; `total`: `number`; \}

### isChecking

> **isChecking**: `boolean`

### isGeneratingHashCodes

> **isGeneratingHashCodes**: `boolean`

### isUploading

> **isUploading**: `boolean`

### processFiles

> **processFiles**: `ProcessFilesFn`

### resetStatus()

> **resetStatus**: () => `void`

#### Returns

`void`

### uploadProgress

> **uploadProgress**: `number`
