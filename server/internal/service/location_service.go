package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"server/internal/db/catalogtx"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/pipeline"
	"server/internal/settings"
)

const (
	geocoderProviderDisabled  = settings.GeocodingProviderDisabled
	geocoderProviderNominatim = settings.GeocodingProviderNominatim
	maxGeocodeClustersPerRun  = 25
	maxProviderAttempts       = 8
	maxReverseGeocodeBody     = 1 << 20
	reverseGeocodeCacheTTL    = 30 * 24 * time.Hour

	// Each reconciliation transaction performs at most this many indexed
	// membership statements plus one cluster-row mutation. Eight transactions
	// per River turn yield between quanta so other foreground and background
	// writers retain admission opportunities.
	maxLocationMembershipMutationsPerTransaction = 16
	maxLocationWriteTransactionsPerTurn          = 8
)

var errLocationClusterSnapshotChanged = errors.New("location cluster source revision changed")

// ErrLocationProjectionStale is returned when a prepared projection was
// computed against an obsolete source or geocoding revision. It is a
// successful stale no-op at the queue boundary.
var ErrLocationProjectionStale = errLocationClusterSnapshotChanged

type LocationService interface {
	// RebuildLocationClusters performs one bounded River turn. MoreWork asks the
	// worker to snooze the same unique job instead of completing it.
	RebuildLocationClusters(ctx context.Context, repositoryID *string, ownerID *int32) (moreWork bool, err error)
	ResolveLocationClusters(ctx context.Context, geocodingRevision int64) (time.Duration, error)
	// PrepareLocationRebuild computes a bounded, immutable projection mutation
	// from a reader snapshot. The commit coordinator applies the returned value.
	PrepareLocationRebuild(ctx context.Context, repositoryID uuid.UUID, ownerID int32, expectedRevision uint64) (PreparedLocationRebuild, error)
	ApplyPreparedLocationRebuildTx(ctx context.Context, tx *sql.Tx, prepared PreparedLocationRebuild) error
	// PrepareLocationResolution performs read-only cache/provider work. Its
	// immutable result is acknowledged through the commit coordinator after the
	// external index/provider work has completed.
	PrepareLocationResolution(ctx context.Context, geocodingRevision int64) (PreparedLocationResolution, error)
	ApplyPreparedLocationResolutionTx(ctx context.Context, tx *sql.Tx, prepared PreparedLocationResolution, projectionVersion uint64) error
	RequestLocationRebuild(ctx context.Context, repositoryID *string, ownerID *int32) (uuid.UUID, error)
	ListLocationClusters(ctx context.Context, params ListLocationClustersParams) ([]LocationCluster, int64, error)
}

func (s *locationService) RequestLocationRebuild(ctx context.Context, repositoryID *string, ownerID *int32) (uuid.UUID, error) {
	receiptID := uuid.New()
	err := s.writer.Transact(ctx, catalogtx.OperationLocationRebuildRequest, nil, func(tx *sql.Tx) error {
		now := time.Now().UTC().UnixMicro()
		subject := "all"
		if repositoryID != nil {
			subject = *repositoryID
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO catalog_operation_receipts(receipt_id,kind,subject_id,desired_version,state,created_at,updated_at) VALUES(?,'rebuild',?,1,'pending',?,?)`, receiptID.String(), subject, now, now); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT repository_id,owner_id FROM location_projection_state WHERE (? IS NULL OR repository_id=?) AND (? IS NULL OR owner_id=?) ORDER BY repository_id,owner_id`, repositoryID, repositoryID, ownerID, ownerID)
		if err != nil {
			return err
		}
		type scope struct {
			repository uuid.UUID
			owner      int32
		}
		var scopes []scope
		for rows.Next() {
			var item scope
			if err := rows.Scan(&item.repository, &item.owner); err != nil {
				_ = rows.Close()
				return err
			}
			scopes = append(scopes, item)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		for _, item := range scopes {
			var revision uint64
			if err := tx.QueryRowContext(ctx, `UPDATE location_projection_state SET source_revision=source_revision+1,updated_at=? WHERE repository_id=? AND owner_id=? RETURNING source_revision`, now, item.repository.String(), item.owner).Scan(&revision); err != nil {
				return err
			}
			if err := pipeline.RequestLocationProjectionTx(ctx, tx, item.repository, item.owner, receiptID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO location_projection_receipt_scopes(receipt_id,repository_id,owner_id,desired_revision) VALUES(?,?,?,?)`, receiptID.String(), item.repository.String(), item.owner, revision); err != nil {
				return err
			}
		}
		if len(scopes) == 0 {
			_, err := tx.ExecContext(ctx, `UPDATE catalog_operation_receipts SET state='completed',applied_version=desired_version,updated_at=? WHERE receipt_id=?`, now, receiptID.String())
			return err
		}
		return nil
	})
	return receiptID, err
}

type ListLocationClustersParams struct {
	RepositoryID *string
	OwnerID      *int32
	Geohash      *string
	Limit        int
	Offset       int
}

type LocationCluster struct {
	ClusterID         string
	OwnerID           *int32
	RepositoryID      string
	Geohash           string
	Precision         int32
	CentroidLatitude  float64
	CentroidLongitude float64
	PhotoCount        int32
	Label             *string
	Country           *string
	Region            *string
	City              *string
	Provider          *string
	GeocodeStatus     string
	GeocodedAt        *time.Time
}

type ReverseGeocodeResult struct {
	Label       *string
	Country     *string
	Region      *string
	City        *string
	RawResponse []byte
}

type ReverseGeocoder interface {
	Provider() string
	Language() string
	Reverse(ctx context.Context, latitude, longitude float64) (ReverseGeocodeResult, error)
}

type locationService struct {
	queries  *repo.Queries
	writer   *catalogtx.Writer
	snapshot *catalogtx.Reader
	pacer    *requestPacer
}

func NewLocationService(queries *repo.Queries, pool *sql.DB, _ ...any) LocationService {
	return NewLocationServiceWithCatalog(
		queries,
		catalogtx.NewWriter(pool, nil),
		catalogtx.NewReader(pool, nil),
	)
}

func NewLocationServiceWithWriter(queries *repo.Queries, pool *sql.DB, writer *catalogtx.Writer, _ ...any) LocationService {
	return NewLocationServiceWithCatalog(queries, writer, catalogtx.NewReader(pool, nil))
}

// NewLocationServiceWithCatalog wires the live catalog's explicit writer and
// query-only snapshot capabilities. Planning a topology never consumes the
// sole writer connection.
func NewLocationServiceWithCatalog(
	queries *repo.Queries,
	writer *catalogtx.Writer,
	snapshot *catalogtx.Reader,
) LocationService {
	return &locationService{
		queries: queries, writer: writer, snapshot: snapshot,
		pacer: &requestPacer{},
	}
}

type desiredLocationCluster struct {
	geohash      string
	latitudeSum  float64
	longitudeSum float64
	assetIDs     map[uuid.UUID]struct{}
}

func (cluster *desiredLocationCluster) centroidLatitude() float64 {
	return cluster.latitudeSum / float64(len(cluster.assetIDs))
}

func (cluster *desiredLocationCluster) centroidLongitude() float64 {
	return cluster.longitudeSum / float64(len(cluster.assetIDs))
}

type storedLocationCluster struct {
	row      repo.LocationCluster
	assetIDs map[uuid.UUID]struct{}
}

type locationClusterMutation struct {
	clusterID            uuid.UUID
	desired              *desiredLocationCluster
	create               bool
	deleteAssetIDs       []uuid.UUID
	insertAssetIDs       []uuid.UUID
	updateTopology       bool
	deleteClusterOnEmpty bool
}

type locationRebuildPlan struct {
	repositoryID      uuid.UUID
	ownerID           int32
	sourceRevision    int64
	publishedRevision int64
	mutations         []*locationClusterMutation
}

// PreparedLocationClusterMutation is the immutable wire value handed from
// projection compute to the commit coordinator. Membership work is capped at
// the configured per-transaction quantum; a later macro turn plans the
// remainder from the now-current catalog state.
type PreparedLocationClusterMutation struct {
	ClusterID            uuid.UUID
	Geohash              string
	CentroidLatitude     float64
	CentroidLongitude    float64
	PhotoCount           int64
	Create               bool
	DeleteAssetIDs       []uuid.UUID
	InsertAssetIDs       []uuid.UUID
	UpdateTopology       bool
	DeleteClusterOnEmpty bool
}

// PreparedLocationRebuild is a complete or bounded prefix of one repository /
// owner projection rebuild. Complete advances published_revision and requests
// geocoding in the same catalog transaction; incomplete prefixes only apply
// their cluster mutations and are followed by another macro turn.
type PreparedLocationRebuild struct {
	RepositoryID      uuid.UUID
	OwnerID           int32
	SourceRevision    int64
	PublishedRevision int64
	Mutations         []PreparedLocationClusterMutation
	Complete          bool
	Noop              bool
}

// PreparedLocationResolutionItem contains one deterministic geocoding result
// or retry transition. Raw provider bytes are immutable compute output and are
// persisted by the coordinator only after revision validation.
type PreparedLocationResolutionItem struct {
	ClusterID     uuid.UUID
	Geohash       string
	Latitude      float64
	Longitude     float64
	SourceKey     string
	Provider      string
	Language      string
	Status        string
	Label         *string
	Country       *string
	Region        *string
	City          *string
	RawResponse   []byte
	AttemptCount  int64
	NextAttemptAt time.Time
}

// PreparedLocationResolution is a bounded provider/index projection batch.
// Complete means no eligible pending cluster remains for this revision after
// applying Items; NextDelay is advisory and keeps the public service method's
// snooze contract for existing callers.
type PreparedLocationResolution struct {
	Revision  int64
	Items     []PreparedLocationResolutionItem
	Complete  bool
	NextDelay time.Duration
}

// PrepareLocationRebuild reads one stable repository/owner snapshot and
// returns only the bounded mutation prefix that can be committed in this
// macro turn. It never acquires the catalog writer.
func (s *locationService) PrepareLocationRebuild(ctx context.Context, repositoryID uuid.UUID, ownerID int32, expectedRevision uint64) (PreparedLocationRebuild, error) {
	if repositoryID == uuid.Nil || ownerID <= 0 || expectedRevision == 0 {
		return PreparedLocationRebuild{}, errors.New("invalid location projection fence")
	}
	plan, err := s.loadLocationRebuildPlan(ctx, repositoryID, ownerID)
	if err != nil {
		return PreparedLocationRebuild{}, err
	}
	if plan == nil {
		return PreparedLocationRebuild{RepositoryID: repositoryID, OwnerID: ownerID, SourceRevision: int64(expectedRevision), Complete: true, Noop: true}, nil
	}
	if plan.sourceRevision != int64(expectedRevision) {
		return PreparedLocationRebuild{}, errLocationClusterSnapshotChanged
	}
	if plan.sourceRevision <= plan.publishedRevision && len(plan.mutations) == 0 {
		return PreparedLocationRebuild{
			RepositoryID: repositoryID, OwnerID: ownerID,
			SourceRevision: plan.sourceRevision, PublishedRevision: plan.publishedRevision,
			Complete: true, Noop: true,
		}, nil
	}
	prepared := PreparedLocationRebuild{
		RepositoryID: repositoryID, OwnerID: ownerID,
		SourceRevision: plan.sourceRevision, PublishedRevision: plan.publishedRevision,
		Complete: true,
	}
	for _, mutation := range plan.mutations {
		deleteIDs := append([]uuid.UUID(nil), mutation.deleteAssetIDs...)
		insertIDs := append([]uuid.UUID(nil), mutation.insertAssetIDs...)
		first := true
		for first || len(deleteIDs) > 0 || len(insertIDs) > 0 {
			if len(prepared.Mutations) >= maxLocationWriteTransactionsPerTurn {
				prepared.Complete = false
				return prepared, nil
			}
			deleteBatch, insertBatch := nextLocationMembershipBatch(deleteIDs, insertIDs)
			completeMutation := len(deleteBatch) == len(deleteIDs) && len(insertBatch) == len(insertIDs)
			item := PreparedLocationClusterMutation{ClusterID: mutation.clusterID, Create: first && mutation.create}
			if mutation.desired != nil {
				item.Geohash = mutation.desired.geohash
				item.CentroidLatitude = mutation.desired.centroidLatitude()
				item.CentroidLongitude = mutation.desired.centroidLongitude()
				item.PhotoCount = int64(len(mutation.desired.assetIDs))
			}
			item.DeleteAssetIDs = append([]uuid.UUID(nil), deleteBatch...)
			item.InsertAssetIDs = append([]uuid.UUID(nil), insertBatch...)
			if completeMutation {
				item.DeleteClusterOnEmpty = mutation.deleteClusterOnEmpty
				item.UpdateTopology = mutation.updateTopology && !mutation.create
			}
			prepared.Mutations = append(prepared.Mutations, item)
			deleteIDs = deleteIDs[len(deleteBatch):]
			insertIDs = insertIDs[len(insertBatch):]
			first = false
		}
	}
	return prepared, nil
}

// ApplyPreparedLocationRebuildTx applies one immutable location mutation
// prefix to a caller-owned transaction. It is the only asynchronous location
// topology write boundary; the public service method below delegates here for
// foreground tests and operational callers.
func (s *locationService) ApplyPreparedLocationRebuildTx(ctx context.Context, tx *sql.Tx, prepared PreparedLocationRebuild) error {
	if s == nil || tx == nil || prepared.RepositoryID == uuid.Nil || prepared.OwnerID <= 0 || prepared.SourceRevision <= 0 {
		return errors.New("location projection commit is incomplete")
	}
	qtx := s.queries.WithTx(tx)
	state, err := qtx.GetLocationProjectionState(ctx, repo.GetLocationProjectionStateParams{RepositoryID: prepared.RepositoryID, OwnerID: prepared.OwnerID})
	if err != nil {
		return fmt.Errorf("validate location source revision: %w", err)
	}
	if state.SourceRevision != prepared.SourceRevision {
		return errLocationClusterSnapshotChanged
	}
	if prepared.Noop {
		return nil
	}
	now := dbtypes.NewTimestamp(time.Now().UTC())
	for _, mutation := range prepared.Mutations {
		if mutation.ClusterID == uuid.Nil {
			return errors.New("location mutation has no cluster identity")
		}
		if mutation.Create {
			provider, status, err := locationTopologyGeocodingState(ctx, qtx)
			if err != nil {
				return err
			}
			owner := prepared.OwnerID
			if _, err := qtx.CreateLocationCluster(ctx, repo.CreateLocationClusterParams{
				ClusterID: mutation.ClusterID, OwnerID: &owner, RepositoryID: prepared.RepositoryID,
				Geohash: mutation.Geohash, Precision: 7,
				CentroidLatitude: mutation.CentroidLatitude, CentroidLongitude: mutation.CentroidLongitude,
				PhotoCount: mutation.PhotoCount, Provider: provider, GeocodeStatus: status,
				CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("create location cluster: %w", err)
			}
		}
		for _, assetID := range mutation.DeleteAssetIDs {
			if _, err := qtx.DeleteLocationClusterAsset(ctx, repo.DeleteLocationClusterAssetParams{ClusterID: mutation.ClusterID, AssetID: assetID}); err != nil {
				return fmt.Errorf("delete location cluster member: %w", err)
			}
		}
		for _, assetID := range mutation.InsertAssetIDs {
			if _, err := qtx.InsertLocationClusterAsset(ctx, repo.InsertLocationClusterAssetParams{ClusterID: mutation.ClusterID, AssetID: assetID, CreatedAt: now}); err != nil {
				return fmt.Errorf("insert location cluster member: %w", err)
			}
		}
		if mutation.DeleteClusterOnEmpty {
			affected, err := qtx.DeleteLocationClusterIfEmpty(ctx, mutation.ClusterID)
			if err != nil {
				return fmt.Errorf("delete stale location cluster: %w", err)
			}
			if affected != 1 {
				return fmt.Errorf("delete stale location cluster %s: affected=%d", mutation.ClusterID, affected)
			}
		}
		if mutation.UpdateTopology {
			provider, status, err := locationTopologyGeocodingState(ctx, qtx)
			if err != nil {
				return err
			}
			affected, err := qtx.UpdateLocationClusterTopology(ctx, repo.UpdateLocationClusterTopologyParams{
				CentroidLatitude: mutation.CentroidLatitude, CentroidLongitude: mutation.CentroidLongitude,
				PhotoCount: mutation.PhotoCount, Provider: provider, GeocodeStatus: status,
				UpdatedAt: now, ClusterID: mutation.ClusterID,
			})
			if err != nil {
				return fmt.Errorf("update location cluster topology: %w", err)
			}
			if affected != 1 {
				return fmt.Errorf("update location cluster %s: affected=%d", mutation.ClusterID, affected)
			}
		}
	}
	if !prepared.Complete {
		return nil
	}
	affected, err := qtx.PublishLocationProjectionRevision(ctx, repo.PublishLocationProjectionRevisionParams{
		SourceRevision: prepared.SourceRevision, UpdatedAt: now,
		RepositoryID: prepared.RepositoryID, OwnerID: prepared.OwnerID,
	})
	if err != nil {
		return fmt.Errorf("publish location projection revision: %w", err)
	}
	if affected != 1 {
		return errLocationClusterSnapshotChanged
	}
	settingsRow, err := qtx.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("read geocoding settings for location publish: %w", err)
	}
	geocoding, err := normalizeStoredGeocoding(settingsRow)
	if err != nil {
		return fmt.Errorf("normalize geocoding settings for location publish: %w", err)
	}
	if geocoding.IsEnabled() {
		if err := pipeline.RequestLocationResolutionTx(ctx, tx, uint64(settingsRow.GeocodingRevision), uuid.New()); err != nil {
			return fmt.Errorf("enqueue location cluster resolution: %w", err)
		}
	}
	return nil
}

// RebuildLocationClusters performs a bounded, revision-fenced reconciliation
// turn. A nil owner is a manual coordinator request: it advances each matching
// concrete scope and transactionally enqueues the child jobs. Concrete jobs
// plan from one query-only WAL snapshot, mutate at most fixed row/transaction
// quanta, and publish only if the source revision is still current.
func (s *locationService) RebuildLocationClusters(ctx context.Context, repositoryID *string, ownerID *int32) (bool, error) {
	repositoryUUID, err := parseOptionalUUID(repositoryID)
	if err != nil {
		return false, err
	}
	if ownerID == nil {
		return false, s.requestLocationClusterRebuilds(ctx, repositoryUUID)
	}
	if !repositoryUUID.Valid {
		return false, errors.New("concrete location rebuild requires a repository")
	}

	for attempt := 0; attempt < 3; attempt++ {
		state, err := s.queries.GetLocationProjectionState(ctx, repo.GetLocationProjectionStateParams{RepositoryID: repositoryUUID.UUID, OwnerID: *ownerID})
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		prepared, prepareErr := s.PrepareLocationRebuild(ctx, repositoryUUID.UUID, *ownerID, uint64(state.SourceRevision))
		if errors.Is(prepareErr, errLocationClusterSnapshotChanged) {
			continue
		}
		if prepareErr != nil {
			return false, prepareErr
		}
		if prepared.Noop {
			return false, nil
		}
		tx, beginErr := s.writer.BeginTx(ctx, catalogtx.OperationLocationRebuildApplyBatch, nil)
		if beginErr != nil {
			return false, fmt.Errorf("begin location rebuild commit: %w", beginErr)
		}
		applyErr := s.ApplyPreparedLocationRebuildTx(ctx, tx.Raw(), prepared)
		if applyErr == nil {
			applyErr = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if errors.Is(applyErr, errLocationClusterSnapshotChanged) {
			continue
		}
		if applyErr != nil {
			return false, applyErr
		}
		return !prepared.Complete, nil
	}
	// Continuous factual churn is expected during import. Yield the River turn
	// instead of consuming retry attempts; the same unique job will re-plan.
	return true, nil
}

func (s *locationService) requestLocationClusterRebuilds(ctx context.Context, repositoryID uuid.NullUUID) error {
	var repositoryFilter any
	if repositoryID.Valid {
		repositoryFilter = repositoryID.UUID
	}
	scopes, err := s.queries.ListLocationProjectionScopes(ctx, repositoryFilter)
	if err != nil {
		return fmt.Errorf("list location projection scopes: %w", err)
	}
	for _, scope := range scopes {
		tx, err := s.writer.BeginTx(ctx, catalogtx.OperationLocationRebuildRequest, nil)
		if err != nil {
			return fmt.Errorf("begin location rebuild request: %w", err)
		}
		qtx := s.queries.WithTx(tx.Raw())
		affected, err := qtx.MarkLocationProjectionScopeDirty(ctx, repo.MarkLocationProjectionScopeDirtyParams{
			UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()), RepositoryID: scope.RepositoryID, OwnerID: scope.OwnerID,
		})
		if err == nil && affected != 1 {
			err = fmt.Errorf("mark location projection scope dirty: affected=%d", affected)
		}
		if err == nil {
			err = pipeline.RequestLocationProjectionTx(ctx, tx.Raw(), scope.RepositoryID, scope.OwnerID, uuid.New())
		}
		if err == nil {
			err = tx.Commit()
		} else {
			_ = tx.Rollback()
		}
		if err != nil {
			return fmt.Errorf("request location rebuild for %s/%d: %w", scope.RepositoryID, scope.OwnerID, err)
		}
	}
	return nil
}

func (s *locationService) loadLocationRebuildPlan(ctx context.Context, repositoryID uuid.UUID, ownerID int32) (*locationRebuildPlan, error) {
	readTx, err := s.snapshot.BeginTx(ctx, catalogtx.OperationLocationRebuildSnapshot)
	if err != nil {
		return nil, fmt.Errorf("begin location rebuild snapshot: %w", err)
	}
	defer readTx.Rollback()
	qtx := s.queries.WithTx(readTx.Raw())

	state, err := qtx.GetLocationProjectionState(ctx, repo.GetLocationProjectionStateParams{
		RepositoryID: repositoryID, OwnerID: ownerID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		if err := readTx.Commit(); err != nil {
			return nil, fmt.Errorf("close empty location rebuild snapshot: %w", err)
		}
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read location projection revision: %w", err)
	}

	owner := ownerID
	desiredRows, err := qtx.ListDesiredLocationClusterMembersForScope(ctx, repo.ListDesiredLocationClusterMembersForScopeParams{
		RepositoryID: repositoryID, OwnerID: &owner,
	})
	if err != nil {
		return nil, fmt.Errorf("list desired location cluster members: %w", err)
	}
	storedRows, err := qtx.ListStoredLocationClustersForScope(ctx, repo.ListStoredLocationClustersForScopeParams{
		RepositoryID: repositoryID, OwnerID: &owner,
	})
	if err != nil {
		return nil, fmt.Errorf("list stored location clusters: %w", err)
	}
	membershipRows, err := qtx.ListStoredLocationClusterAssetsForScope(ctx, repo.ListStoredLocationClusterAssetsForScopeParams{
		RepositoryID: repositoryID, OwnerID: &owner,
	})
	if err != nil {
		return nil, fmt.Errorf("list stored location cluster members: %w", err)
	}
	if err := readTx.Commit(); err != nil {
		return nil, fmt.Errorf("commit location rebuild snapshot: %w", err)
	}

	desired := make(map[string]*desiredLocationCluster)
	for _, row := range desiredRows {
		if row.Geohash == nil || row.Latitude == nil || row.Longitude == nil {
			return nil, errors.New("location candidate contains an unexpected null GPS fact")
		}
		cluster := desired[*row.Geohash]
		if cluster == nil {
			cluster = &desiredLocationCluster{geohash: *row.Geohash, assetIDs: make(map[uuid.UUID]struct{})}
			desired[*row.Geohash] = cluster
		}
		if _, duplicate := cluster.assetIDs[row.AssetID]; duplicate {
			continue
		}
		cluster.assetIDs[row.AssetID] = struct{}{}
		cluster.latitudeSum += *row.Latitude
		cluster.longitudeSum += *row.Longitude
	}

	storedByGeohash := make(map[string]*storedLocationCluster, len(storedRows))
	storedByID := make(map[uuid.UUID]*storedLocationCluster, len(storedRows))
	for _, row := range storedRows {
		if _, duplicate := storedByGeohash[row.Geohash]; duplicate {
			return nil, fmt.Errorf("duplicate stored location cluster for geohash %q", row.Geohash)
		}
		cluster := &storedLocationCluster{row: row, assetIDs: make(map[uuid.UUID]struct{})}
		storedByGeohash[row.Geohash] = cluster
		storedByID[row.ClusterID] = cluster
	}
	for _, membership := range membershipRows {
		cluster := storedByID[membership.ClusterID]
		if cluster == nil {
			return nil, fmt.Errorf("stored location membership references unknown cluster %s", membership.ClusterID)
		}
		cluster.assetIDs[membership.AssetID] = struct{}{}
	}

	keys := make([]string, 0, len(desired)+len(storedByGeohash))
	seen := make(map[string]struct{}, len(desired)+len(storedByGeohash))
	for key := range desired {
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	for key := range storedByGeohash {
		if _, exists := seen[key]; !exists {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	plan := &locationRebuildPlan{
		repositoryID: repositoryID, ownerID: ownerID,
		sourceRevision: state.SourceRevision, publishedRevision: state.PublishedRevision,
	}
	for _, key := range keys {
		desiredCluster := desired[key]
		storedCluster := storedByGeohash[key]
		mutation := &locationClusterMutation{desired: desiredCluster}
		switch {
		case desiredCluster == nil:
			mutation.clusterID = storedCluster.row.ClusterID
			mutation.deleteAssetIDs = sortedLocationAssetIDs(storedCluster.assetIDs)
			mutation.deleteClusterOnEmpty = true
		case storedCluster == nil:
			mutation.clusterID = uuid.New()
			mutation.create = true
			mutation.insertAssetIDs = sortedLocationAssetIDs(desiredCluster.assetIDs)
		case desiredCluster != nil:
			mutation.clusterID = storedCluster.row.ClusterID
			mutation.deleteAssetIDs = sortedLocationAssetDifference(storedCluster.assetIDs, desiredCluster.assetIDs)
			mutation.insertAssetIDs = sortedLocationAssetDifference(desiredCluster.assetIDs, storedCluster.assetIDs)
			mutation.updateTopology = len(mutation.deleteAssetIDs) > 0 || len(mutation.insertAssetIDs) > 0 ||
				storedCluster.row.PhotoCount != int64(len(desiredCluster.assetIDs)) ||
				math.Abs(storedCluster.row.CentroidLatitude-desiredCluster.centroidLatitude()) > 1e-12 ||
				math.Abs(storedCluster.row.CentroidLongitude-desiredCluster.centroidLongitude()) > 1e-12
		}
		if mutation.create || mutation.deleteClusterOnEmpty || mutation.updateTopology ||
			len(mutation.deleteAssetIDs) > 0 || len(mutation.insertAssetIDs) > 0 {
			plan.mutations = append(plan.mutations, mutation)
		}
	}
	return plan, nil
}

func sortedLocationAssetIDs(values map[uuid.UUID]struct{}) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	return ids
}

func sortedLocationAssetDifference(left, right map[uuid.UUID]struct{}) []uuid.UUID {
	values := make(map[uuid.UUID]struct{})
	for id := range left {
		if _, exists := right[id]; !exists {
			values[id] = struct{}{}
		}
	}
	return sortedLocationAssetIDs(values)
}

func nextLocationMembershipBatch(deletes, inserts []uuid.UUID) ([]uuid.UUID, []uuid.UUID) {
	deleteCount := min(len(deletes), maxLocationMembershipMutationsPerTransaction)
	remaining := maxLocationMembershipMutationsPerTransaction - deleteCount
	insertCount := min(len(inserts), remaining)
	return deletes[:deleteCount], inserts[:insertCount]
}

func locationTopologyGeocodingState(ctx context.Context, qtx *repo.Queries) (*string, string, error) {
	settingsRow, err := qtx.GetSettings(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("read geocoding settings for location topology: %w", err)
	}
	geocoding, err := normalizeStoredGeocoding(settingsRow)
	if err != nil {
		return nil, "", fmt.Errorf("normalize geocoding settings for location topology: %w", err)
	}
	if geocoding.IsEnabled() {
		return nil, "pending", nil
	}
	provider := geocoderProviderDisabled
	return &provider, "disabled", nil
}

func (s *locationService) ListLocationClusters(ctx context.Context, params ListLocationClustersParams) ([]LocationCluster, int64, error) {
	repositoryUUID, err := parseOptionalUUID(params.RepositoryID)
	if err != nil {
		return nil, 0, err
	}

	total, err := s.queries.CountLocationClusters(ctx, repo.CountLocationClustersParams{
		RepositoryID: repositoryUUID,
		OwnerID:      params.OwnerID,
		Geohash:      normalizeOptionalText(params.Geohash),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count location clusters: %w", err)
	}

	rows, err := s.queries.ListLocationClusters(ctx, repo.ListLocationClustersParams{
		RepositoryID: repositoryUUID,
		OwnerID:      params.OwnerID,
		Geohash:      normalizeOptionalText(params.Geohash),
		Limit:        int64(params.Limit),
		Offset:       int64(params.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list location clusters: %w", err)
	}

	clusters := make([]LocationCluster, 0, len(rows))
	for _, row := range rows {
		clusters = append(clusters, toLocationCluster(row))
	}
	return clusters, total, nil
}

// ResolveLocationClusters performs one bounded durable batch. A positive
// duration tells the River worker to snooze the same job until the next
// eligible cluster rather than completing and hoping a successor is inserted.
func (s *locationService) ResolveLocationClusters(ctx context.Context, geocodingRevision int64) (time.Duration, error) {
	prepared, err := s.PrepareLocationResolution(ctx, geocodingRevision)
	if err != nil {
		return 0, err
	}
	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationLocationRemotePublish, nil)
	if err != nil {
		return 0, fmt.Errorf("begin location resolution commit: %w", err)
	}
	if err := s.ApplyPreparedLocationResolutionTx(ctx, tx.Raw(), prepared, 0); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit location resolution: %w", err)
	}
	return prepared.NextDelay, nil
}

// PrepareLocationResolution performs all bounded reader/provider work without
// mutating catalog state. The resulting items can safely be retried after a
// crash because provider responses are immutable and catalog application is
// revision-fenced.
func (s *locationService) PrepareLocationResolution(ctx context.Context, geocodingRevision int64) (PreparedLocationResolution, error) {
	if geocodingRevision <= 0 {
		return PreparedLocationResolution{}, errors.New("invalid geocoding revision")
	}
	settingsRow, err := s.queries.GetSettings(ctx)
	if err != nil {
		return PreparedLocationResolution{}, fmt.Errorf("load geocoding settings: %w", err)
	}
	geocoding, err := normalizeStoredGeocoding(settingsRow)
	if err != nil {
		return PreparedLocationResolution{}, fmt.Errorf("load geocoding settings: %w", err)
	}
	prepared := PreparedLocationResolution{Revision: geocodingRevision, Complete: true}
	if settingsRow.GeocodingRevision != geocodingRevision || !geocoding.IsEnabled() {
		return prepared, nil
	}
	if current, err := s.revisionIsCurrent(ctx, geocodingRevision); err != nil {
		return PreparedLocationResolution{}, err
	} else if !current {
		return PreparedLocationResolution{}, errLocationClusterSnapshotChanged
	}
	now := time.Now().UTC()
	clusters, err := s.queries.ListPendingLocationClusters(ctx, repo.ListPendingLocationClustersParams{
		Now: dbtypes.NewTimestamp(now), RepositoryID: nil, OwnerID: nil,
		Limit: maxGeocodeClustersPerRun + 1,
	})
	if err != nil {
		return PreparedLocationResolution{}, fmt.Errorf("list pending location clusters: %w", err)
	}
	moreEligible := len(clusters) > maxGeocodeClustersPerRun
	if moreEligible {
		clusters = clusters[:maxGeocodeClustersPerRun]
	}
	geocoder := newReverseGeocoder(geocoding, s.pacer)
	for _, cluster := range clusters {
		if current, err := s.revisionIsCurrent(ctx, geocodingRevision); err != nil {
			return PreparedLocationResolution{}, err
		} else if !current {
			return PreparedLocationResolution{}, errLocationClusterSnapshotChanged
		}
		item := PreparedLocationResolutionItem{
			ClusterID: cluster.ClusterID, Geohash: cluster.Geohash,
			Latitude: cluster.CentroidLatitude, Longitude: cluster.CentroidLongitude,
			SourceKey: geocoding.SourceKey(), Provider: geocoding.Provider, Language: geocoding.Language,
		}
		cached, cacheErr := s.queries.GetReverseGeocodeCache(ctx, repo.GetReverseGeocodeCacheParams{
			SourceKey: item.SourceKey, Geohash: item.Geohash, Provider: item.Provider, Language: item.Language,
			Now: dbtypes.NewTimestamp(time.Now().UTC()),
		})
		if cacheErr == nil {
			item.Status, item.Label, item.Country, item.Region, item.City = "cached", cached.Label, cached.Country, cached.Region, cached.City
			prepared.Items = append(prepared.Items, item)
			continue
		}
		if !errors.Is(cacheErr, sql.ErrNoRows) {
			return PreparedLocationResolution{}, fmt.Errorf("get reverse geocode cache: %w", cacheErr)
		}
		result, providerErr := geocoder.Reverse(ctx, cluster.CentroidLatitude, cluster.CentroidLongitude)
		if providerErr != nil {
			if ctx.Err() != nil {
				return PreparedLocationResolution{}, ctx.Err()
			}
			var typed *geocodeProviderError
			if !errors.As(providerErr, &typed) {
				typed = &geocodeProviderError{retryable: true, cause: providerErr}
			}
			attempt := cluster.GeocodeAttemptCount + 1
			item.Status, item.AttemptCount = "pending", attempt
			if !typed.retryable || attempt >= maxProviderAttempts {
				item.Status = "failed"
			} else {
				item.NextAttemptAt = time.Now().UTC().Add(providerRetryDelay(attempt, typed.retryAfter))
			}
			prepared.Items = append(prepared.Items, item)
			continue
		}
		item.Status = "resolved"
		item.Label, item.Country, item.Region, item.City = result.Label, result.Country, result.Region, result.City
		item.RawResponse = append([]byte(nil), result.RawResponse...)
		prepared.Items = append(prepared.Items, item)
	}
	schedule, err := s.queries.GetPendingLocationClusterSchedule(ctx)
	if err != nil {
		return PreparedLocationResolution{}, fmt.Errorf("schedule pending location clusters: %w", err)
	}
	remaining := moreEligible || schedule.PendingCount > int64(len(prepared.Items))
	for _, item := range prepared.Items {
		if item.Status == "pending" {
			remaining = true
			break
		}
	}
	prepared.Complete = !remaining
	if remaining {
		nextAttemptAt := scheduleUnixMicro(schedule.NextAttemptAt)
		if nextAttemptAt <= 0 {
			prepared.NextDelay = time.Second
		} else {
			prepared.NextDelay = time.Until(time.UnixMicro(nextAttemptAt))
			if prepared.NextDelay < time.Second {
				prepared.NextDelay = time.Second
			}
		}
	}
	return prepared, nil
}

// ApplyPreparedLocationResolutionTx persists provider results and retry state,
// then advances the projection ledger when the prepared batch is complete.
func (s *locationService) ApplyPreparedLocationResolutionTx(ctx context.Context, tx *sql.Tx, prepared PreparedLocationResolution, projectionVersion uint64) error {
	if s == nil || tx == nil || prepared.Revision <= 0 {
		return errors.New("location resolution commit is incomplete")
	}
	qtx := s.queries.WithTx(tx)
	settingsRow, err := qtx.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("check geocoding revision before publication: %w", err)
	}
	if settingsRow.GeocodingRevision != prepared.Revision {
		return errLocationClusterSnapshotChanged
	}
	geocoding, err := normalizeStoredGeocoding(settingsRow)
	if err != nil {
		return fmt.Errorf("normalize geocoding settings before publication: %w", err)
	}
	if !geocoding.IsEnabled() {
		if projectionVersion > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE location_resolution_pipeline_state SET applied_revision=projection_version,terminal_error=NULL,updated_at=? WHERE scope='all' AND source_revision=? AND projection_version=? AND applied_revision<projection_version`, time.Now().UTC().UnixMicro(), prepared.Revision, projectionVersion); err != nil {
				return fmt.Errorf("advance disabled location resolution projection: %w", err)
			}
		}
		return nil
	}
	now := time.Now().UTC()
	for _, item := range prepared.Items {
		if item.ClusterID == uuid.Nil {
			return errors.New("location resolution item has no cluster identity")
		}
		switch item.Status {
		case "cached":
			provider := item.Provider
			if _, err := qtx.UpdateLocationClusterGeocodeIfRevision(ctx, repo.UpdateLocationClusterGeocodeIfRevisionParams{
				Label: item.Label, Country: item.Country, Region: item.Region, City: item.City,
				Provider: &provider, GeocodeStatus: "cached", GeocodedAt: dbtypes.NewTimestamp(now),
				GeocodeAttemptCount: 0, GeocodeNextAttemptAt: dbtypes.Timestamp{}, ClusterID: item.ClusterID, GeocodingRevision: prepared.Revision,
			}); err != nil {
				return fmt.Errorf("publish cached location result: %w", err)
			}
		case "resolved":
			if _, err := qtx.UpsertReverseGeocodeCache(ctx, repo.UpsertReverseGeocodeCacheParams{
				SourceKey: item.SourceKey, Geohash: item.Geohash, Provider: item.Provider, Language: item.Language,
				Latitude: item.Latitude, Longitude: item.Longitude, Label: item.Label, Country: item.Country,
				Region: item.Region, City: item.City, RawResponse: dbtypes.JSON(item.RawResponse),
				QueriedAt: dbtypes.NewTimestamp(now), ExpiresAt: dbtypes.NewTimestamp(now.Add(reverseGeocodeCacheTTL)),
			}); err != nil {
				return fmt.Errorf("cache reverse geocode result: %w", err)
			}
			provider := item.Provider
			if _, err := qtx.UpdateLocationClusterGeocodeIfRevision(ctx, repo.UpdateLocationClusterGeocodeIfRevisionParams{
				Label: item.Label, Country: item.Country, Region: item.Region, City: item.City,
				Provider: &provider, GeocodeStatus: "resolved", GeocodedAt: dbtypes.NewTimestamp(now),
				GeocodeAttemptCount: 0, GeocodeNextAttemptAt: dbtypes.Timestamp{}, ClusterID: item.ClusterID, GeocodingRevision: prepared.Revision,
			}); err != nil {
				return fmt.Errorf("publish location result: %w", err)
			}
		case "pending", "failed":
			if _, err := qtx.UpdateLocationClusterRetryIfRevision(ctx, repo.UpdateLocationClusterRetryIfRevisionParams{
				GeocodeStatus: item.Status, GeocodeAttemptCount: item.AttemptCount,
				GeocodeNextAttemptAt: timestampOrZero(item.NextAttemptAt), GeocodedAt: dbtypes.NewTimestamp(now),
				ClusterID: item.ClusterID, GeocodingRevision: prepared.Revision,
			}); err != nil {
				return fmt.Errorf("persist location provider retry: %w", err)
			}
		default:
			return fmt.Errorf("unsupported prepared location status %q", item.Status)
		}
	}
	if !prepared.Complete || projectionVersion == 0 {
		return nil
	}
	result, err := tx.ExecContext(ctx, `UPDATE location_resolution_pipeline_state SET applied_revision=projection_version,terminal_error=NULL,updated_at=? WHERE scope='all' AND source_revision=? AND projection_version=? AND applied_revision<projection_version`, now.UnixMicro(), prepared.Revision, projectionVersion)
	if err != nil {
		return fmt.Errorf("advance location resolution projection: %w", err)
	}
	_ = result
	return nil
}

func scheduleUnixMicro(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	case dbtypes.Timestamp:
		if typed.Valid {
			return typed.Time.UnixMicro()
		}
	}
	return 0
}

func timestampOrZero(value time.Time) dbtypes.Timestamp {
	if value.IsZero() {
		return dbtypes.Timestamp{}
	}
	return dbtypes.NewTimestamp(value)
}

func (s *locationService) revisionIsCurrent(ctx context.Context, revision int64) (bool, error) {
	row, err := s.queries.GetSettings(ctx)
	if err != nil {
		return false, fmt.Errorf("check geocoding revision: %w", err)
	}
	return row.GeocodingRevision == revision, nil
}

func providerRetryDelay(attempt int64, retryAfter time.Duration) time.Duration {
	delay := 5 * time.Second
	for index := int64(1); index < attempt; index++ {
		if delay >= 5*time.Minute {
			delay = 5 * time.Minute
			break
		}
		delay *= 2
	}
	if delay > 5*time.Minute {
		delay = 5 * time.Minute
	}
	if retryAfter > delay {
		delay = retryAfter
	}
	if delay > time.Hour {
		delay = time.Hour
	}
	return delay
}

func parseOptionalUUID(raw *string) (uuid.NullUUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return uuid.NullUUID{}, nil
	}
	parsed, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return uuid.NullUUID{}, fmt.Errorf("invalid repository ID: %w", err)
	}
	return uuid.NullUUID{UUID: parsed, Valid: true}, nil
}

func normalizeOptionalText(raw *string) *string {
	if raw == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func toLocationCluster(row repo.LocationCluster) LocationCluster {
	var geocodedAt *time.Time
	if row.GeocodedAt.Valid {
		t := row.GeocodedAt.Time
		geocodedAt = &t
	}
	return LocationCluster{
		ClusterID:         row.ClusterID.String(),
		OwnerID:           row.OwnerID,
		RepositoryID:      row.RepositoryID.String(),
		Geohash:           row.Geohash,
		Precision:         int32(row.Precision),
		CentroidLatitude:  row.CentroidLatitude,
		CentroidLongitude: row.CentroidLongitude,
		PhotoCount:        int32(row.PhotoCount),
		Label:             row.Label,
		Country:           row.Country,
		Region:            row.Region,
		City:              row.City,
		Provider:          row.Provider,
		GeocodeStatus:     row.GeocodeStatus,
		GeocodedAt:        geocodedAt,
	}
}

type disabledGeocoder struct{}

func (disabledGeocoder) Provider() string { return geocoderProviderDisabled }
func (disabledGeocoder) Language() string { return "" }
func (disabledGeocoder) Reverse(context.Context, float64, float64) (ReverseGeocodeResult, error) {
	return ReverseGeocodeResult{}, &geocodeProviderError{cause: errors.New("reverse geocoder disabled")}
}

type requestPacer struct {
	mu        sync.Mutex
	lastStart time.Time
}

func (p *requestPacer) acquire(ctx context.Context) (func(), error) {
	p.mu.Lock()
	wait := time.Until(p.lastStart.Add(time.Second))
	if wait > 0 {
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			p.mu.Unlock()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	p.lastStart = time.Now()
	return p.mu.Unlock, nil
}

type nominatimGeocoder struct {
	endpoint   string
	language   string
	userAgent  string
	httpClient *http.Client
	pacer      *requestPacer
}

func newReverseGeocoder(cfg settings.Geocoding, pacers ...*requestPacer) ReverseGeocoder {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if provider != geocoderProviderNominatim {
		return disabledGeocoder{}
	}
	pacer := &requestPacer{}
	if len(pacers) != 0 && pacers[0] != nil {
		pacer = pacers[0]
	}
	return &nominatimGeocoder{
		endpoint:   strings.TrimSpace(cfg.NominatimEndpoint),
		language:   strings.TrimSpace(cfg.Language),
		userAgent:  strings.TrimSpace(cfg.UserAgent),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		pacer:      pacer,
	}
}

func (g *nominatimGeocoder) Provider() string { return geocoderProviderNominatim }
func (g *nominatimGeocoder) Language() string { return g.language }

func (g *nominatimGeocoder) Reverse(ctx context.Context, latitude, longitude float64) (ReverseGeocodeResult, error) {
	baseURL, err := url.Parse(g.endpoint)
	if err != nil {
		return ReverseGeocodeResult{}, &geocodeProviderError{cause: fmt.Errorf("invalid nominatim endpoint: %w", err)}
	}
	query := baseURL.Query()
	query.Set("format", "jsonv2")
	query.Set("lat", fmt.Sprintf("%.8f", latitude))
	query.Set("lon", fmt.Sprintf("%.8f", longitude))
	query.Set("zoom", "14")
	query.Set("addressdetails", "1")
	if g.language != "" {
		query.Set("accept-language", g.language)
	}
	baseURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), nil)
	if err != nil {
		return ReverseGeocodeResult{}, &geocodeProviderError{cause: err}
	}
	req.Header.Set("User-Agent", g.userAgent)
	release, err := g.pacer.acquire(ctx)
	if err != nil {
		return ReverseGeocodeResult{}, &geocodeProviderError{retryable: true, cause: err}
	}
	defer release()

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return ReverseGeocodeResult{}, &geocodeProviderError{retryable: true, cause: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ReverseGeocodeResult{}, &geocodeProviderError{
			retryable:  resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500,
			retryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
			statusCode: resp.StatusCode,
			cause:      fmt.Errorf("nominatim returned status class %dxx", resp.StatusCode/100),
		}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReverseGeocodeBody+1))
	if err != nil {
		return ReverseGeocodeResult{}, &geocodeProviderError{retryable: true, cause: err}
	}
	if len(body) > maxReverseGeocodeBody {
		return ReverseGeocodeResult{}, &geocodeProviderError{cause: errors.New("nominatim response exceeds the configured size limit")}
	}

	var parsed struct {
		DisplayName string            `json:"display_name"`
		Name        string            `json:"name"`
		Address     map[string]string `json:"address"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return ReverseGeocodeResult{}, &geocodeProviderError{cause: fmt.Errorf("decode nominatim response: %w", err)}
	}

	label := firstNonEmpty(parsed.DisplayName, parsed.Name)
	country := firstNonEmpty(parsed.Address["country"])
	region := firstNonEmpty(parsed.Address["state"], parsed.Address["region"], parsed.Address["province"])
	city := firstNonEmpty(parsed.Address["city"], parsed.Address["town"], parsed.Address["village"], parsed.Address["municipality"], parsed.Address["county"])

	return ReverseGeocodeResult{
		Label:       emptyStringToNil(label),
		Country:     emptyStringToNil(country),
		Region:      emptyStringToNil(region),
		City:        emptyStringToNil(city),
		RawResponse: body,
	}, nil
}

type geocodeProviderError struct {
	retryable  bool
	retryAfter time.Duration
	statusCode int
	cause      error
}

func (e *geocodeProviderError) Error() string {
	if e.cause == nil {
		return "reverse geocoding provider failed"
	}
	return e.cause.Error()
}

func (e *geocodeProviderError) Unwrap() error { return e.cause }

func parseRetryAfter(raw string) time.Duration {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds >= 0 {
		if seconds >= int64(time.Hour/time.Second) {
			return time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(raw)
	if err != nil {
		return 0
	}
	if delay := time.Until(when); delay > 0 {
		return minDuration(delay, time.Hour)
	}
	return 0
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func emptyStringToNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
