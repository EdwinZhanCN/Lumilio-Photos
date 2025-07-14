[lumilio-web](../../../../modules.md) / [hooks/api-hooks/useUploadProcess](../index.md) / useUploadProcess

# Function: useUploadProcess()

> **useUploadProcess**(`workerClientRef`, `wasmReady`): `object`

Defined in: [hooks/api-hooks/useUploadProcess.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/87d62aab38919e216231c72a6e5a6bce24754b5d/web/src/hooks/api-hooks/useUploadProcess.tsx#L26)

## Parameters

### workerClientRef

`RefObject`\<`any`\>

### wasmReady

`boolean`

## Returns

`object`

### hashcodeProgress

> **hashcodeProgress**: `null` \| \{ `error?`: `string`; `failedAt?`: `number`; `numberProcessed?`: `number`; `total?`: `number`; \}

### isGeneratingHashCodes

> **isGeneratingHashCodes**: `boolean`

### isUploading

> **isUploading**: `boolean` = `uploadMutation.isPending`

### processFiles

> **processFiles**: `ProcessFilesFn`

### resetStatus()

> **resetStatus**: () => `void`

#### Returns

`void`

### uploadProgress

> **uploadProgress**: `number`
