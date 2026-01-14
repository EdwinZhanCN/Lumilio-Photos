[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useExportImage](../index.md) / useExportImageReturn

# Interface: useExportImageReturn

Defined in: [hooks/util-hooks/useExportImage.tsx:26](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a1e668df4214942756ee5b246e79ddcc4607c48e/web/src/hooks/util-hooks/useExportImage.tsx#L26)

## Properties

### cancelExport()

> **cancelExport**: () => `void`

Defined in: [hooks/util-hooks/useExportImage.tsx:36](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a1e668df4214942756ee5b246e79ddcc4607c48e/web/src/hooks/util-hooks/useExportImage.tsx#L36)

#### Returns

`void`

***

### downloadOriginal()

> **downloadOriginal**: (`asset`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useExportImage.tsx:29](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a1e668df4214942756ee5b246e79ddcc4607c48e/web/src/hooks/util-hooks/useExportImage.tsx#L29)

#### Parameters

##### asset

###### asset_id?

`string`

###### deleted_at?

`string`

###### duration?

`number`

###### file_size?

`number`

###### hash?

`string`

###### height?

`number`

###### is_deleted?

`boolean`

###### liked?

`boolean`

###### mime_type?

`string`

###### original_filename?

`string`

###### owner_id?

`number`

###### rating?

`number`

###### repository_id?

`string`

###### specific_metadata?

\{ `camera_model?`: `string`; `description?`: `string`; `dimensions?`: `string`; `exposure?`: `number`; `exposure_time?`: `string`; `f_number?`: `number`; `focal_length?`: `number`; `gps_latitude?`: `number`; `gps_longitude?`: `number`; `is_raw?`: `boolean`; `iso_speed?`: `number`; `lens_model?`: `string`; `resolution?`: `string`; `taken_time?`: `string`; \} \| \{ `bitrate?`: `number`; `camera_model?`: `string`; `codec?`: `string`; `description?`: `string`; `frame_rate?`: `number`; `gps_latitude?`: `number`; `gps_longitude?`: `number`; `recorded_time?`: `string`; \} \| \{ `album?`: `string`; `artist?`: `string`; `bitrate?`: `number`; `channels?`: `number`; `codec?`: `string`; `description?`: `string`; `genre?`: `string`; `sample_rate?`: `number`; `title?`: `string`; `year?`: `number`; \}

###### status?

`number`[]

###### storage_path?

`string`

###### taken_time?

`string`

###### type?

`string`

###### upload_time?

`string`

###### width?

`number`

#### Returns

`Promise`\<`void`\>

***

### exportImage()

> **exportImage**: (`asset`, `options`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useExportImage.tsx:30](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a1e668df4214942756ee5b246e79ddcc4607c48e/web/src/hooks/util-hooks/useExportImage.tsx#L30)

#### Parameters

##### asset

###### asset_id?

`string`

###### deleted_at?

`string`

###### duration?

`number`

###### file_size?

`number`

###### hash?

`string`

###### height?

`number`

###### is_deleted?

`boolean`

###### liked?

`boolean`

###### mime_type?

`string`

###### original_filename?

`string`

###### owner_id?

`number`

###### rating?

`number`

###### repository_id?

`string`

###### specific_metadata?

\{ `camera_model?`: `string`; `description?`: `string`; `dimensions?`: `string`; `exposure?`: `number`; `exposure_time?`: `string`; `f_number?`: `number`; `focal_length?`: `number`; `gps_latitude?`: `number`; `gps_longitude?`: `number`; `is_raw?`: `boolean`; `iso_speed?`: `number`; `lens_model?`: `string`; `resolution?`: `string`; `taken_time?`: `string`; \} \| \{ `bitrate?`: `number`; `camera_model?`: `string`; `codec?`: `string`; `description?`: `string`; `frame_rate?`: `number`; `gps_latitude?`: `number`; `gps_longitude?`: `number`; `recorded_time?`: `string`; \} \| \{ `album?`: `string`; `artist?`: `string`; `bitrate?`: `number`; `channels?`: `number`; `codec?`: `string`; `description?`: `string`; `genre?`: `string`; `sample_rate?`: `number`; `title?`: `string`; `year?`: `number`; \}

###### status?

`number`[]

###### storage_path?

`string`

###### taken_time?

`string`

###### type?

`string`

###### upload_time?

`string`

###### width?

`number`

##### options

[`ExportOptions`](ExportOptions.md)

#### Returns

`Promise`\<`void`\>

***

### exportMultiple()

> **exportMultiple**: (`assets`, `options`, `priority?`) => `Promise`\<`void`\>

Defined in: [hooks/util-hooks/useExportImage.tsx:31](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a1e668df4214942756ee5b246e79ddcc4607c48e/web/src/hooks/util-hooks/useExportImage.tsx#L31)

#### Parameters

##### assets

`object`[]

##### options

[`ExportOptions`](ExportOptions.md)

##### priority?

[`ProcessingPriority`](../../../../utils/smartBatchSizing/enumerations/ProcessingPriority.md)

#### Returns

`Promise`\<`void`\>

***

### exportProgress

> **exportProgress**: `null` \| [`ExportProgress`](ExportProgress.md)

Defined in: [hooks/util-hooks/useExportImage.tsx:28](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a1e668df4214942756ee5b246e79ddcc4607c48e/web/src/hooks/util-hooks/useExportImage.tsx#L28)

***

### isExporting

> **isExporting**: `boolean`

Defined in: [hooks/util-hooks/useExportImage.tsx:27](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/a1e668df4214942756ee5b246e79ddcc4607c48e/web/src/hooks/util-hooks/useExportImage.tsx#L27)
