[lumilio-web](../../../modules.md) / [utils/smartBatchSizing](../index.md) / SmartBatchSizer

# Class: SmartBatchSizer

Defined in: [utils/smartBatchSizing.ts:145](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/utils/smartBatchSizing.ts#L145)

Smart Batch Sizing Manager
Maintains performance history and adapts batch sizes dynamically

## Constructors

### Constructor

> **new SmartBatchSizer**(): `SmartBatchSizer`

Defined in: [utils/smartBatchSizing.ts:151](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/utils/smartBatchSizing.ts#L151)

#### Returns

`SmartBatchSizer`

## Methods

### getDeviceCapabilities()

> **getDeviceCapabilities**(): [`DeviceCapabilities`](../interfaces/DeviceCapabilities.md)

Defined in: [utils/smartBatchSizing.ts:363](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/utils/smartBatchSizing.ts#L363)

Gets device capabilities

#### Returns

[`DeviceCapabilities`](../interfaces/DeviceCapabilities.md)

***

### getOptimalBatchSize()

> **getOptimalBatchSize**(`operationType`, `totalItems`, `priority`): `number`

Defined in: [utils/smartBatchSizing.ts:158](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/utils/smartBatchSizing.ts#L158)

Gets the optimal batch size for a given operation type and priority

#### Parameters

##### operationType

`string`

##### totalItems

`number`

##### priority

[`ProcessingPriority`](../enumerations/ProcessingPriority.md) = `ProcessingPriority.NORMAL`

#### Returns

`number`

***

### isMemoryPressureDetected()

> **isMemoryPressureDetected**(): `boolean`

Defined in: [utils/smartBatchSizing.ts:224](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/utils/smartBatchSizing.ts#L224)

Checks if memory pressure is detected

#### Returns

`boolean`

***

### recordMetrics()

> **recordMetrics**(`metrics`): `void`

Defined in: [utils/smartBatchSizing.ts:202](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/utils/smartBatchSizing.ts#L202)

Records processing metrics for future batch size optimization

#### Parameters

##### metrics

[`ProcessingMetrics`](../interfaces/ProcessingMetrics.md)

#### Returns

`void`

***

### resetMetrics()

> **resetMetrics**(): `void`

Defined in: [utils/smartBatchSizing.ts:370](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/96695ff97a9c30bb49d2a37326e8e3aec3dc4c19/web/src/utils/smartBatchSizing.ts#L370)

Resets metrics history (useful for testing or configuration changes)

#### Returns

`void`
