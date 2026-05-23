[lumilio-web](../../../../modules.md) / [hooks/util-hooks/useExportImage](../index.md) / useExportImage

# Function: useExportImage()

> **useExportImage**(): [`useExportImageReturn`](../interfaces/useExportImageReturn.md)

Defined in: [hooks/util-hooks/useExportImage.tsx:49](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/951cf1b7fa322fefddc97d3fa89023d9d5095996/web/src/hooks/util-hooks/useExportImage.tsx#L49)

Custom hook for downloading and exporting images.
It uses the shared worker client for format conversion and processing.
This hook must be used within a component tree wrapped by `<WorkerProvider />`.

## Returns

[`useExportImageReturn`](../interfaces/useExportImageReturn.md)

Hook state and actions for image export.

## Author

Edwin Zhan

## Since

1.1.0
