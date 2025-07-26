[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useExportImage](../index.md) / useExportImageReturn

# Interface: useExportImageReturn

Defined in: [hooks/util-hooks/useExportImage.tsx:20](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/b3425e71774123fa6b07d528432324028816c356/web/src/hooks/util-hooks/useExportImage.tsx#L20)

## Properties

### cancelExport()

> **cancelExport**: () => `void`

Defined in: [hooks/util-hooks/useExportImage.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/b3425e71774123fa6b07d528432324028816c356/web/src/hooks/util-hooks/useExportImage.tsx#L26)

#### Returns

`void`

***

### downloadOriginal()

> **downloadOriginal**: (`asset`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useExportImage.tsx:23](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/b3425e71774123fa6b07d528432324028816c356/web/src/hooks/util-hooks/useExportImage.tsx#L23)

#### Parameters

##### asset

`Asset`

#### Returns

`Promise`\<`void`\>

***

### exportImage()

> **exportImage**: (`asset`, `options`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useExportImage.tsx:24](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/b3425e71774123fa6b07d528432324028816c356/web/src/hooks/util-hooks/useExportImage.tsx#L24)

#### Parameters

##### asset

`Asset`

##### options

[`ExportOptions`](ExportOptions.md)

#### Returns

`Promise`\<`void`\>

***

### exportMultiple()

> **exportMultiple**: (`assets`, `options`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useExportImage.tsx:25](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/b3425e71774123fa6b07d528432324028816c356/web/src/hooks/util-hooks/useExportImage.tsx#L25)

#### Parameters

##### assets

`Asset`[]

##### options

[`ExportOptions`](ExportOptions.md)

#### Returns

`Promise`\<`void`\>

***

### exportProgress

> **exportProgress**: `null` \| [`ExportProgress`](ExportProgress.md)

Defined in: [hooks/util-hooks/useExportImage.tsx:22](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/b3425e71774123fa6b07d528432324028816c356/web/src/hooks/util-hooks/useExportImage.tsx#L22)

***

### isExporting

> **isExporting**: `boolean`

Defined in: [hooks/util-hooks/useExportImage.tsx:21](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/b3425e71774123fa6b07d528432324028816c356/web/src/hooks/util-hooks/useExportImage.tsx#L21)
