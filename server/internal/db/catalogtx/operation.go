// Package catalogtx owns the closed, low-cardinality names and measured
// transaction capabilities for Lumilio's SQLite catalog.
package catalogtx

import "fmt"

// Role identifies which physical catalog capability owns an operation.
type Role uint8

const (
	RoleUnknown Role = iota
	RoleWriter
	RoleReader
)

func (r Role) String() string {
	switch r {
	case RoleWriter:
		return "writer"
	case RoleReader:
		return "reader"
	default:
		return "unknown"
	}
}

func (r Role) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// OperationKind prevents a statement-only label from being used to disguise a
// transaction (or vice versa). ApplicationTransaction is the zero value so the
// intentionally large application catalog stays readable; invalid operations
// still fail closed through their unknown role.
type OperationKind uint8

const (
	OperationKindApplicationTransaction OperationKind = iota
	OperationKindStatement
	OperationKindDriverTransaction
)

func (k OperationKind) String() string {
	switch k {
	case OperationKindApplicationTransaction:
		return "application_transaction"
	case OperationKindStatement:
		return "statement"
	case OperationKindDriverTransaction:
		return "driver_transaction"
	default:
		return "unknown"
	}
}

func (k OperationKind) MarshalText() ([]byte, error) { return []byte(k.String()), nil }

// Operation is an index into the compile-time catalog below. It deliberately
// has no constructor from a string: request, repository, asset, and job data
// cannot become metric labels.
type Operation uint16

const (
	OperationInvalid Operation = iota
	OperationRepositoryObservationClaim
	OperationRepositoryObservationRequest
	OperationRepositoryObservationCancelRequest
	OperationRepositoryObservationStart
	OperationRepositoryObservationApplyDirectoryBatch
	OperationRepositoryObservationApplyChangeBatch
	OperationRepositoryObservationFinalizeAbsence
	OperationRepositoryObservationFinish
	OperationRepositoryObservationFailFrontier
	OperationRepositoryObservationCancelRun
	OperationRepositoryMaterializeKnownContent
	OperationRepositoryMaterializeHash
	OperationRepositoryReobserveNode
	OperationRepositoryAssetActivate
	OperationRepositoryRelocate
	OperationRepositoryRemove
	OperationRepositoryRootRelocateMaintenance
	OperationRepositoryRootRelocate
	OperationRepositoryRootDelete
	OperationRepositoryLifecycleComplete
	OperationRepositoryLifecycleFail
	OperationRepositoryHostActionFinish
	OperationAssetMetadataPublish
	OperationAssetDelete
	OperationAssetRestore
	OperationAssetStagingCommit
	OperationAssetReprocess
	OperationAssetReindexRequest
	OperationAssetUserStateMutate
	OperationEventInitializeBackfill
	OperationEventRebuildSnapshot
	OperationEventPublishOwnerSnapshot
	OperationEventMerge
	OperationEventSplit
	OperationEventRemoveMember
	OperationEventAddAssets
	OperationEventRebuildClaim
	OperationEventRebuildFinalize
	OperationEventRebuildLeaseRenew
	OperationEventRebuildRequest
	OperationEventRebuildState
	OperationEventShare
	OperationLocationRebuildSnapshot
	OperationLocationRebuildRequest
	OperationLocationRebuildApplyBatch
	OperationLocationRebuildPublish
	OperationLocationRemotePublish
	OperationFaceMutation
	OperationFaceClusterResetScope
	OperationFaceClusterPublishPending
	OperationFaceClusterRefresh
	OperationEmbeddingSave
	OperationVideoFrameEmbeddingSave
	OperationEmbeddingSearchSpaceResolve
	OperationVectorRepairDerived
	OperationVectorRebuildFlat
	OperationVectorTrainANN
	OperationDuplicateDetectPublish
	OperationDuplicateMerge
	OperationStackStructuralMerge
	OperationStackBurstPublish
	OperationStackManualCreate
	OperationStackRemoveMember
	OperationStackDelete
	OperationStackLivePhotoMatch
	OperationOCRSave
	OperationOCRDelete
	OperationOCRIndexBatch
	OperationUserMutation
	OperationAuthMutation
	OperationSettingsGeocodingUpdate
	OperationAgentEffectCommit
	OperationAgentEffectCleanup
	OperationAgentBindRun
	OperationAgentTransitionAwaitingRun
	OperationAgentFinishRun
	OperationEventPatch
	OperationEventSchedulerLeaseCleanup
	OperationClassifierSavePrototypes
	OperationRepositoryHostActionMutation
	OperationRepositoryLifecycleMarkScanQueued
	OperationSQLitePassiveCheckpoint
	OperationSQLiteOptimize
	OperationSQLiteTruncateCheckpoint
	OperationVectorStateRepair
	OperationVectorClearPending
	OperationBackgroundCommitBatch
	OperationDomainOutboxDeliver
	OperationDomainOutboxReconcile
	OperationBackupRequest
	OperationCatalogGeneratedWriterExec
	OperationCatalogGeneratedWriterReturning
	OperationCatalogGeneratedReaderRows
	OperationCatalogUnknownWriterStatement
	OperationCatalogUnknownReaderStatement
	OperationCatalogRawWriterTransaction
	operationCount
)

// OperationDescriptor is the stable external representation of one bounded
// catalog operation.
type OperationDescriptor struct {
	Operation Operation     `json:"-"`
	Name      string        `json:"name"`
	Role      Role          `json:"role"`
	Kind      OperationKind `json:"kind"`
}

var operationCatalog = [operationCount]OperationDescriptor{
	OperationInvalid: {
		Operation: OperationInvalid,
		Name:      "invalid",
		Role:      RoleUnknown,
	},
	OperationBackgroundCommitBatch: {
		Operation: OperationBackgroundCommitBatch,
		Name:      "background.commit.batch",
		Role:      RoleWriter,
	},
	OperationDomainOutboxDeliver: {
		Operation: OperationDomainOutboxDeliver,
		Name:      "domain_outbox.deliver",
		Role:      RoleWriter,
	},
	OperationDomainOutboxReconcile: {
		Operation: OperationDomainOutboxReconcile,
		Name:      "domain_outbox.reconcile",
		Role:      RoleWriter,
	},
	OperationBackupRequest: {
		Operation: OperationBackupRequest,
		Name:      "backup.request",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationClaim: {
		Operation: OperationRepositoryObservationClaim,
		Name:      "repository.observe.claim",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationRequest: {
		Operation: OperationRepositoryObservationRequest,
		Name:      "repository.observe.request",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationCancelRequest: {
		Operation: OperationRepositoryObservationCancelRequest,
		Name:      "repository.observe.cancel_request",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationStart: {
		Operation: OperationRepositoryObservationStart,
		Name:      "repository.observe.start",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationApplyDirectoryBatch: {
		Operation: OperationRepositoryObservationApplyDirectoryBatch,
		Name:      "repository.observe.apply_directory_batch",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationApplyChangeBatch: {
		Operation: OperationRepositoryObservationApplyChangeBatch,
		Name:      "repository.observe.apply_change_batch",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationFinalizeAbsence: {
		Operation: OperationRepositoryObservationFinalizeAbsence,
		Name:      "repository.observe.finalize_absence",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationFinish: {
		Operation: OperationRepositoryObservationFinish,
		Name:      "repository.observe.finish",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationFailFrontier: {
		Operation: OperationRepositoryObservationFailFrontier,
		Name:      "repository.observe.fail_frontier",
		Role:      RoleWriter,
	},
	OperationRepositoryObservationCancelRun: {
		Operation: OperationRepositoryObservationCancelRun,
		Name:      "repository.observe.cancel_run",
		Role:      RoleWriter,
	},
	OperationRepositoryMaterializeKnownContent: {
		Operation: OperationRepositoryMaterializeKnownContent,
		Name:      "repository.materialize.known_content",
		Role:      RoleWriter,
	},
	OperationRepositoryMaterializeHash: {
		Operation: OperationRepositoryMaterializeHash,
		Name:      "repository.materialize.hash",
		Role:      RoleWriter,
	},
	OperationRepositoryReobserveNode: {
		Operation: OperationRepositoryReobserveNode,
		Name:      "repository.materialize.reobserve_node",
		Role:      RoleWriter,
	},
	OperationRepositoryAssetActivate: {
		Operation: OperationRepositoryAssetActivate,
		Name:      "repository.asset.activate",
		Role:      RoleWriter,
	},
	OperationRepositoryRelocate: {
		Operation: OperationRepositoryRelocate,
		Name:      "repository.relocate",
		Role:      RoleWriter,
	},
	OperationRepositoryRemove: {
		Operation: OperationRepositoryRemove,
		Name:      "repository.remove",
		Role:      RoleWriter,
	},
	OperationRepositoryRootRelocateMaintenance: {
		Operation: OperationRepositoryRootRelocateMaintenance,
		Name:      "repository_root.relocate_maintenance",
		Role:      RoleWriter,
	},
	OperationRepositoryRootRelocate: {
		Operation: OperationRepositoryRootRelocate,
		Name:      "repository_root.relocate",
		Role:      RoleWriter,
	},
	OperationRepositoryRootDelete: {
		Operation: OperationRepositoryRootDelete,
		Name:      "repository_root.delete",
		Role:      RoleWriter,
	},
	OperationRepositoryLifecycleComplete: {
		Operation: OperationRepositoryLifecycleComplete,
		Name:      "repository.lifecycle.complete",
		Role:      RoleWriter,
	},
	OperationRepositoryLifecycleFail: {
		Operation: OperationRepositoryLifecycleFail,
		Name:      "repository.lifecycle.fail",
		Role:      RoleWriter,
	},
	OperationRepositoryHostActionFinish: {
		Operation: OperationRepositoryHostActionFinish,
		Name:      "repository.host_action.finish",
		Role:      RoleWriter,
	},
	OperationAssetMetadataPublish: {
		Operation: OperationAssetMetadataPublish,
		Name:      "asset.metadata.publish",
		Role:      RoleWriter,
	},
	OperationAssetDelete: {
		Operation: OperationAssetDelete,
		Name:      "asset.delete",
		Role:      RoleWriter,
	},
	OperationAssetRestore: {
		Operation: OperationAssetRestore,
		Name:      "asset.restore",
		Role:      RoleWriter,
	},
	OperationAssetStagingCommit: {
		Operation: OperationAssetStagingCommit,
		Name:      "asset.staging_commit.enqueue",
		Role:      RoleWriter,
	},
	OperationAssetReprocess: {
		Operation: OperationAssetReprocess,
		Name:      "asset.reprocess.enqueue",
		Role:      RoleWriter,
	},
	OperationAssetReindexRequest: {
		Operation: OperationAssetReindexRequest,
		Name:      "asset.reindex.request",
		Role:      RoleWriter,
	},
	OperationAssetUserStateMutate: {
		Operation: OperationAssetUserStateMutate,
		Name:      "asset.user_state.mutate",
		Role:      RoleWriter,
	},
	OperationEventInitializeBackfill: {
		Operation: OperationEventInitializeBackfill,
		Name:      "event.initialize_backfill",
		Role:      RoleWriter,
	},
	OperationEventRebuildSnapshot: {
		Operation: OperationEventRebuildSnapshot,
		Name:      "event.rebuild.snapshot",
		Role:      RoleReader,
	},
	OperationEventPublishOwnerSnapshot: {
		Operation: OperationEventPublishOwnerSnapshot,
		Name:      "event.publish_owner_snapshot",
		Role:      RoleWriter,
	},
	OperationEventMerge: {
		Operation: OperationEventMerge,
		Name:      "event.merge",
		Role:      RoleWriter,
	},
	OperationEventSplit: {
		Operation: OperationEventSplit,
		Name:      "event.split",
		Role:      RoleWriter,
	},
	OperationEventRemoveMember: {
		Operation: OperationEventRemoveMember,
		Name:      "event.remove_member",
		Role:      RoleWriter,
	},
	OperationEventAddAssets: {
		Operation: OperationEventAddAssets,
		Name:      "event.add_assets",
		Role:      RoleWriter,
	},
	OperationEventRebuildClaim: {
		Operation: OperationEventRebuildClaim,
		Name:      "event.rebuild.claim",
		Role:      RoleWriter,
	},
	OperationEventRebuildFinalize: {
		Operation: OperationEventRebuildFinalize,
		Name:      "event.rebuild.finalize",
		Role:      RoleWriter,
	},
	OperationEventRebuildLeaseRenew: {
		Operation: OperationEventRebuildLeaseRenew,
		Name:      "event.rebuild.renew_lease",
		Role:      RoleWriter,
	},
	OperationEventRebuildRequest: {
		Operation: OperationEventRebuildRequest,
		Name:      "event.rebuild.request",
		Role:      RoleWriter,
	},
	OperationEventRebuildState: {
		Operation: OperationEventRebuildState,
		Name:      "event.rebuild.state",
		Role:      RoleWriter,
	},
	OperationEventShare: {
		Operation: OperationEventShare,
		Name:      "event.share",
		Role:      RoleWriter,
	},
	OperationLocationRebuildSnapshot: {
		Operation: OperationLocationRebuildSnapshot,
		Name:      "location.rebuild.snapshot",
		Role:      RoleReader,
	},
	OperationLocationRebuildRequest: {
		Operation: OperationLocationRebuildRequest,
		Name:      "location.rebuild.request",
		Role:      RoleWriter,
	},
	OperationLocationRebuildApplyBatch: {
		Operation: OperationLocationRebuildApplyBatch,
		Name:      "location.rebuild.apply_batch",
		Role:      RoleWriter,
	},
	OperationLocationRebuildPublish: {
		Operation: OperationLocationRebuildPublish,
		Name:      "location.rebuild.publish",
		Role:      RoleWriter,
	},
	OperationLocationRemotePublish: {
		Operation: OperationLocationRemotePublish,
		Name:      "location.remote.publish",
		Role:      RoleWriter,
	},
	OperationFaceMutation: {
		Operation: OperationFaceMutation,
		Name:      "face.mutation",
		Role:      RoleWriter,
	},
	OperationFaceClusterResetScope: {
		Operation: OperationFaceClusterResetScope,
		Name:      "face_cluster.reset_scope",
		Role:      RoleWriter,
	},
	OperationFaceClusterPublishPending: {
		Operation: OperationFaceClusterPublishPending,
		Name:      "face_cluster.publish_pending",
		Role:      RoleWriter,
	},
	OperationFaceClusterRefresh: {
		Operation: OperationFaceClusterRefresh,
		Name:      "face_cluster.refresh",
		Role:      RoleWriter,
	},
	OperationEmbeddingSave: {
		Operation: OperationEmbeddingSave,
		Name:      "embedding.save",
		Role:      RoleWriter,
	},
	OperationVideoFrameEmbeddingSave: {
		Operation: OperationVideoFrameEmbeddingSave,
		Name:      "embedding.video_frames.save",
		Role:      RoleWriter,
	},
	OperationEmbeddingSearchSpaceResolve: {
		Operation: OperationEmbeddingSearchSpaceResolve,
		Name:      "embedding.search_space.resolve",
		Role:      RoleWriter,
	},
	OperationVectorRepairDerived: {
		Operation: OperationVectorRepairDerived,
		Name:      "vector_index.repair_derived",
		Role:      RoleWriter,
	},
	OperationVectorRebuildFlat: {
		Operation: OperationVectorRebuildFlat,
		Name:      "vector_index.rebuild_flat",
		Role:      RoleWriter,
	},
	OperationVectorTrainANN: {
		Operation: OperationVectorTrainANN,
		Name:      "vector_index.train_ann",
		Role:      RoleWriter,
	},
	OperationDuplicateDetectPublish: {
		Operation: OperationDuplicateDetectPublish,
		Name:      "duplicate.detect.publish",
		Role:      RoleWriter,
	},
	OperationDuplicateMerge: {
		Operation: OperationDuplicateMerge,
		Name:      "duplicate.merge",
		Role:      RoleWriter,
	},
	OperationStackStructuralMerge: {
		Operation: OperationStackStructuralMerge,
		Name:      "stack.structural_merge",
		Role:      RoleWriter,
	},
	OperationStackBurstPublish: {
		Operation: OperationStackBurstPublish,
		Name:      "stack.burst.publish",
		Role:      RoleWriter,
	},
	OperationStackManualCreate: {
		Operation: OperationStackManualCreate,
		Name:      "stack.manual.create",
		Role:      RoleWriter,
	},
	OperationStackRemoveMember: {
		Operation: OperationStackRemoveMember,
		Name:      "stack.remove_member",
		Role:      RoleWriter,
	},
	OperationStackDelete: {
		Operation: OperationStackDelete,
		Name:      "stack.delete",
		Role:      RoleWriter,
	},
	OperationStackLivePhotoMatch: {
		Operation: OperationStackLivePhotoMatch,
		Name:      "stack.live_photo.match",
		Role:      RoleWriter,
	},
	OperationOCRSave: {
		Operation: OperationOCRSave,
		Name:      "ocr.save",
		Role:      RoleWriter,
	},
	OperationOCRDelete: {
		Operation: OperationOCRDelete,
		Name:      "ocr.delete",
		Role:      RoleWriter,
	},
	OperationOCRIndexBatch: {
		Operation: OperationOCRIndexBatch,
		Name:      "ocr_index.batch",
		Role:      RoleWriter,
	},
	OperationUserMutation: {
		Operation: OperationUserMutation,
		Name:      "user.mutation",
		Role:      RoleWriter,
	},
	OperationAuthMutation: {
		Operation: OperationAuthMutation,
		Name:      "auth.mutation",
		Role:      RoleWriter,
	},
	OperationSettingsGeocodingUpdate: {
		Operation: OperationSettingsGeocodingUpdate,
		Name:      "settings.geocoding.update",
		Role:      RoleWriter,
	},
	OperationAgentEffectCommit: {
		Operation: OperationAgentEffectCommit,
		Name:      "agent.effect.commit",
		Role:      RoleWriter,
	},
	OperationAgentEffectCleanup: {
		Operation: OperationAgentEffectCleanup,
		Name:      "agent.effect.cleanup",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationAgentBindRun: {
		Operation: OperationAgentBindRun,
		Name:      "agent.run.bind",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationAgentTransitionAwaitingRun: {
		Operation: OperationAgentTransitionAwaitingRun,
		Name:      "agent.run.transition_awaiting",
		Role:      RoleWriter,
	},
	OperationAgentFinishRun: {
		Operation: OperationAgentFinishRun,
		Name:      "agent.run.finish",
		Role:      RoleWriter,
	},
	OperationEventPatch: {
		Operation: OperationEventPatch,
		Name:      "event.patch",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationEventSchedulerLeaseCleanup: {
		Operation: OperationEventSchedulerLeaseCleanup,
		Name:      "event.scheduler.lease_cleanup",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationClassifierSavePrototypes: {
		Operation: OperationClassifierSavePrototypes,
		Name:      "classifier.save_prototypes",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationRepositoryHostActionMutation: {
		Operation: OperationRepositoryHostActionMutation,
		Name:      "repository.host_action.mutation",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationRepositoryLifecycleMarkScanQueued: {
		Operation: OperationRepositoryLifecycleMarkScanQueued,
		Name:      "repository.lifecycle.mark_scan_queued",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationSQLitePassiveCheckpoint: {
		Operation: OperationSQLitePassiveCheckpoint,
		Name:      "sqlite.checkpoint.passive",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationSQLiteOptimize: {
		Operation: OperationSQLiteOptimize,
		Name:      "sqlite.optimize",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationSQLiteTruncateCheckpoint: {
		Operation: OperationSQLiteTruncateCheckpoint,
		Name:      "sqlite.checkpoint.truncate",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationVectorClearPending: {
		Operation: OperationVectorClearPending,
		Name:      "vector_index.clear_pending",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationVectorStateRepair: {
		Operation: OperationVectorStateRepair,
		Name:      "vector_index.state_repair",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationCatalogGeneratedWriterExec: {
		Operation: OperationCatalogGeneratedWriterExec,
		Name:      "catalog.statement.generated_exec",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationCatalogGeneratedWriterReturning: {
		Operation: OperationCatalogGeneratedWriterReturning,
		Name:      "catalog.statement.generated_returning",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationCatalogGeneratedReaderRows: {
		Operation: OperationCatalogGeneratedReaderRows,
		Name:      "catalog.statement.generated_read",
		Role:      RoleReader,
		Kind:      OperationKindStatement,
	},
	OperationCatalogUnknownWriterStatement: {
		Operation: OperationCatalogUnknownWriterStatement,
		Name:      "catalog.statement.unknown_write",
		Role:      RoleWriter,
		Kind:      OperationKindStatement,
	},
	OperationCatalogUnknownReaderStatement: {
		Operation: OperationCatalogUnknownReaderStatement,
		Name:      "catalog.statement.unknown_read",
		Role:      RoleReader,
		Kind:      OperationKindStatement,
	},
	OperationCatalogRawWriterTransaction: {
		Operation: OperationCatalogRawWriterTransaction,
		Name:      "catalog.driver.raw_transaction",
		Role:      RoleWriter,
		Kind:      OperationKindDriverTransaction,
	},
}

// Operations returns a copy of every valid descriptor in declaration order.
func Operations() []OperationDescriptor {
	result := make([]OperationDescriptor, 0, len(operationCatalog)-1)
	for operation := Operation(1); int(operation) < len(operationCatalog); operation++ {
		descriptor := operationCatalog[operation]
		if descriptor.Operation == operation && descriptor.Role != RoleUnknown {
			result = append(result, descriptor)
		}
	}
	return result
}

// Name returns the operation's bounded metric/artifact name.
func (o Operation) Name() string {
	descriptor, ok := o.Descriptor()
	if !ok {
		return "invalid"
	}
	return descriptor.Name
}

// Role returns the physical capability required by the operation.
func (o Operation) Role() Role {
	descriptor, ok := o.Descriptor()
	if !ok {
		return RoleUnknown
	}
	return descriptor.Role
}

// Kind distinguishes application transactions, individual statements, and
// driver-owned transactions such as River's internal catalog scopes.
func (o Operation) Kind() OperationKind {
	descriptor, ok := o.Descriptor()
	if !ok {
		return OperationKind(255)
	}
	return descriptor.Kind
}

// Descriptor resolves a valid operation without accepting a dynamic label.
func (o Operation) Descriptor() (OperationDescriptor, bool) {
	if o == OperationInvalid || int(o) >= len(operationCatalog) {
		return OperationDescriptor{}, false
	}
	descriptor := operationCatalog[o]
	if descriptor.Operation != o || descriptor.Role == RoleUnknown || descriptor.Name == "" {
		return OperationDescriptor{}, false
	}
	return descriptor, true
}

func (o Operation) String() string {
	if descriptor, ok := o.Descriptor(); ok {
		return descriptor.Name
	}
	return fmt.Sprintf("invalid_operation_%d", uint16(o))
}
