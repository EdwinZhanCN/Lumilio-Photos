[lumilio-web](../../../modules.md) / [utils/performancePreferences](../index.md) / PerformancePreferencesManager

# Class: PerformancePreferencesManager

Defined in: [utils/performancePreferences.ts:37](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L37)

Performance Preferences Manager

## Constructors

### Constructor

> **new PerformancePreferencesManager**(): `PerformancePreferencesManager`

Defined in: [utils/performancePreferences.ts:41](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L41)

#### Returns

`PerformancePreferencesManager`

## Methods

### addListener()

> **addListener**(`listener`): () => `void`

Defined in: [utils/performancePreferences.ts:132](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L132)

Adds a listener for preference changes

#### Parameters

##### listener

(`prefs`) => `void`

#### Returns

> (): `void`

##### Returns

`void`

***

### getBatchSizeMultiplier()

> **getBatchSizeMultiplier**(): `number`

Defined in: [utils/performancePreferences.ts:64](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L64)

Gets batch size multiplier based on current profile

#### Returns

`number`

***

### getMaxConcurrentOperations()

> **getMaxConcurrentOperations**(): `number`

Defined in: [utils/performancePreferences.ts:116](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L116)

Gets maximum concurrent operations allowed

#### Returns

`number`

***

### getMemoryConstraintMultiplier()

> **getMemoryConstraintMultiplier**(): `number`

Defined in: [utils/performancePreferences.ts:87](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L87)

Gets memory constraint multiplier

#### Returns

`number`

***

### getPreferences()

> **getPreferences**(): [`PerformancePreferences`](../interfaces/PerformancePreferences.md)

Defined in: [utils/performancePreferences.ts:48](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L48)

Gets current performance preferences

#### Returns

[`PerformancePreferences`](../interfaces/PerformancePreferences.md)

***

### resetToDefaults()

> **resetToDefaults**(): `void`

Defined in: [utils/performancePreferences.ts:123](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L123)

Resets preferences to defaults

#### Returns

`void`

***

### shouldPrioritizeUserOperations()

> **shouldPrioritizeUserOperations**(): `boolean`

Defined in: [utils/performancePreferences.ts:109](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L109)

Checks if priority operations should be enhanced

#### Returns

`boolean`

***

### updatePreferences()

> **updatePreferences**(`updates`): `void`

Defined in: [utils/performancePreferences.ts:55](https://github.com/EdwinZhanCN/Lumilio-Photos/blob/3dd9035b607ebbe85d911491cffd43a9e01c377d/web/src/utils/performancePreferences.ts#L55)

Updates performance preferences

#### Parameters

##### updates

`Partial`\<[`PerformancePreferences`](../interfaces/PerformancePreferences.md)\>

#### Returns

`void`
