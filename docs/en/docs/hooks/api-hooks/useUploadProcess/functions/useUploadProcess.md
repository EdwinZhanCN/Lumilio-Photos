[lumilio-web](../../../../modules.md) / [hooks/api-hooks/useUploadProcess](../index.md) / useUploadProcess

# Function: useUploadProcess()

> **useUploadProcess**(`workerClientRef`, `wasmReady`): `object`

Defined in: [hooks/api-hooks/useUploadProcess.tsx:49](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a7285497a028269d9cd6a31a72019f8b71eba616/web/src/hooks/api-hooks/useUploadProcess.tsx#L49)

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
