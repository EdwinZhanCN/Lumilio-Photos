[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useExportImage](../index.md) / useExportImageReturn

# Interface: useExportImageReturn

Defined in: hooks/util-hooks/useExportImage.tsx:20

## Properties

### cancelExport()

> **cancelExport**: () => `void`

Defined in: hooks/util-hooks/useExportImage.tsx:26

#### Returns

`void`

***

### downloadOriginal()

> **downloadOriginal**: (`asset`) => `Promise`\<`void`\>

Defined in: hooks/util-hooks/useExportImage.tsx:23

#### Parameters

##### asset

`Asset`

#### Returns

`Promise`\<`void`\>

***

### exportImage()

> **exportImage**: (`asset`, `options`) => `Promise`\<`void`\>

Defined in: hooks/util-hooks/useExportImage.tsx:24

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

Defined in: hooks/util-hooks/useExportImage.tsx:25

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

Defined in: hooks/util-hooks/useExportImage.tsx:22

***

### isExporting

> **isExporting**: `boolean`

Defined in: hooks/util-hooks/useExportImage.tsx:21
