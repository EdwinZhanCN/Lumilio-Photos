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
	"server/internal/queue/jobs"
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

type LocationService interface {
	// RebuildLocationClusters performs one bounded River turn. MoreWork asks the
	// worker to snooze the same unique job instead of completing it.
	RebuildLocationClusters(ctx context.Context, repositoryID *string, ownerID *int32) (moreWork bool, err error)
	ResolveLocationClusters(ctx context.Context, geocodingRevision int64) (time.Duration, error)
	ListLocationClusters(ctx context.Context, params ListLocationClustersParams) ([]LocationCluster, int64, error)
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
	queue    RiverJobInserter
	pacer    *requestPacer
}

func NewLocationService(queries *repo.Queries, pool *sql.DB, queue RiverJobInserter) LocationService {
	return NewLocationServiceWithCatalog(
		queries,
		catalogtx.NewWriter(pool, nil),
		catalogtx.NewReader(pool, nil),
		queue,
	)
}

func NewLocationServiceWithWriter(queries *repo.Queries, pool *sql.DB, writer *catalogtx.Writer, queue RiverJobInserter) LocationService {
	return NewLocationServiceWithCatalog(queries, writer, catalogtx.NewReader(pool, nil), queue)
}

// NewLocationServiceWithCatalog wires the live catalog's explicit writer and
// query-only snapshot capabilities. Planning a topology never consumes the
// sole writer connection.
func NewLocationServiceWithCatalog(
	queries *repo.Queries,
	writer *catalogtx.Writer,
	snapshot *catalogtx.Reader,
	queue RiverJobInserter,
) LocationService {
	return &locationService{
		queries: queries, writer: writer, snapshot: snapshot, queue: queue,
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
		plan, planErr := s.loadLocationRebuildPlan(ctx, repositoryUUID.UUID, *ownerID)
		if planErr != nil {
			return false, planErr
		}
		if plan == nil {
			return false, nil
		}
		if plan.sourceRevision <= plan.publishedRevision && len(plan.mutations) == 0 {
			return false, nil
		}
		moreWork, applyErr := s.applyLocationRebuildPlan(ctx, plan)
		if errors.Is(applyErr, errLocationClusterSnapshotChanged) {
			continue
		}
		return moreWork, applyErr
	}
	// Continuous factual churn is expected during import. Yield the River turn
	// instead of consuming retry attempts; the same unique job will re-plan.
	return true, nil
}

func (s *locationService) requestLocationClusterRebuilds(ctx context.Context, repositoryID uuid.NullUUID) error {
	if s.queue == nil {
		return errors.New("location rebuild queue is not configured")
	}
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
			repositoryText := scope.RepositoryID.String()
			owner := scope.OwnerID
			args := jobs.RebuildLocationClustersArgs{RepositoryID: &repositoryText, OwnerID: &owner}
			opts := args.InsertOpts()
			_, err = s.queue.InsertTx(ctx, tx.Raw(), args, &opts)
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

func (s *locationService) applyLocationRebuildPlan(ctx context.Context, plan *locationRebuildPlan) (bool, error) {
	transactions := 0
	for len(plan.mutations) > 0 && transactions < maxLocationWriteTransactionsPerTurn {
		complete, err := s.applyLocationClusterMutation(ctx, plan, plan.mutations[0])
		if err != nil {
			return false, err
		}
		transactions++
		if complete {
			plan.mutations = plan.mutations[1:]
		}
	}
	if len(plan.mutations) > 0 {
		return true, nil
	}
	if err := s.publishLocationProjection(ctx, plan); err != nil {
		return false, err
	}
	return false, nil
}

func (s *locationService) applyLocationClusterMutation(
	ctx context.Context,
	plan *locationRebuildPlan,
	mutation *locationClusterMutation,
) (bool, error) {
	deleteBatch, insertBatch := nextLocationMembershipBatch(mutation.deleteAssetIDs, mutation.insertAssetIDs)
	membersComplete := len(deleteBatch) == len(mutation.deleteAssetIDs) && len(insertBatch) == len(mutation.insertAssetIDs)
	createdThisTurn := mutation.create

	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationLocationRebuildApplyBatch, nil)
	if err != nil {
		return false, fmt.Errorf("begin location rebuild batch: %w", err)
	}
	defer tx.Rollback()
	qtx := s.queries.WithTx(tx.Raw())
	state, err := qtx.GetLocationProjectionState(ctx, repo.GetLocationProjectionStateParams{
		RepositoryID: plan.repositoryID, OwnerID: plan.ownerID,
	})
	if err != nil {
		return false, fmt.Errorf("validate location source revision: %w", err)
	}
	if state.SourceRevision != plan.sourceRevision {
		return false, errLocationClusterSnapshotChanged
	}

	now := dbtypes.NewTimestamp(time.Now().UTC())
	if mutation.create {
		provider, status, err := locationTopologyGeocodingState(ctx, qtx)
		if err != nil {
			return false, err
		}
		owner := plan.ownerID
		if _, err := qtx.CreateLocationCluster(ctx, repo.CreateLocationClusterParams{
			ClusterID: mutation.clusterID, OwnerID: &owner, RepositoryID: plan.repositoryID,
			Geohash: mutation.desired.geohash, Precision: 7,
			CentroidLatitude: mutation.desired.centroidLatitude(), CentroidLongitude: mutation.desired.centroidLongitude(),
			PhotoCount: int64(len(mutation.desired.assetIDs)), Provider: provider, GeocodeStatus: status,
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return false, fmt.Errorf("create location cluster: %w", err)
		}
	}
	for _, assetID := range deleteBatch {
		if _, err := qtx.DeleteLocationClusterAsset(ctx, repo.DeleteLocationClusterAssetParams{
			ClusterID: mutation.clusterID, AssetID: assetID,
		}); err != nil {
			return false, fmt.Errorf("delete location cluster member: %w", err)
		}
	}
	for _, assetID := range insertBatch {
		if _, err := qtx.InsertLocationClusterAsset(ctx, repo.InsertLocationClusterAssetParams{
			ClusterID: mutation.clusterID, AssetID: assetID, CreatedAt: now,
		}); err != nil {
			return false, fmt.Errorf("insert location cluster member: %w", err)
		}
	}
	if membersComplete && mutation.deleteClusterOnEmpty {
		affected, err := qtx.DeleteLocationClusterIfEmpty(ctx, mutation.clusterID)
		if err != nil {
			return false, fmt.Errorf("delete stale location cluster: %w", err)
		}
		if affected != 1 {
			return false, fmt.Errorf("delete stale location cluster %s: affected=%d", mutation.clusterID, affected)
		}
	}
	if membersComplete && mutation.updateTopology && !createdThisTurn {
		provider, status, err := locationTopologyGeocodingState(ctx, qtx)
		if err != nil {
			return false, err
		}
		affected, err := qtx.UpdateLocationClusterTopology(ctx, repo.UpdateLocationClusterTopologyParams{
			CentroidLatitude: mutation.desired.centroidLatitude(), CentroidLongitude: mutation.desired.centroidLongitude(),
			PhotoCount: int64(len(mutation.desired.assetIDs)), Provider: provider, GeocodeStatus: status,
			UpdatedAt: now, ClusterID: mutation.clusterID,
		})
		if err != nil {
			return false, fmt.Errorf("update location cluster topology: %w", err)
		}
		if affected != 1 {
			return false, fmt.Errorf("update location cluster %s: affected=%d", mutation.clusterID, affected)
		}
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit location rebuild batch: %w", err)
	}

	mutation.create = false
	mutation.deleteAssetIDs = mutation.deleteAssetIDs[len(deleteBatch):]
	mutation.insertAssetIDs = mutation.insertAssetIDs[len(insertBatch):]
	return membersComplete, nil
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

func (s *locationService) publishLocationProjection(ctx context.Context, plan *locationRebuildPlan) error {
	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationLocationRebuildPublish, nil)
	if err != nil {
		return fmt.Errorf("begin location projection publish: %w", err)
	}
	defer tx.Rollback()
	qtx := s.queries.WithTx(tx.Raw())
	affected, err := qtx.PublishLocationProjectionRevision(ctx, repo.PublishLocationProjectionRevisionParams{
		SourceRevision: plan.sourceRevision, UpdatedAt: dbtypes.NewTimestamp(time.Now().UTC()),
		RepositoryID: plan.repositoryID, OwnerID: plan.ownerID,
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
		if s.queue == nil {
			return errors.New("location resolver queue is not configured")
		}
		args := jobs.ResolveLocationClustersArgs{GeocodingRevision: settingsRow.GeocodingRevision}
		opts := args.InsertOpts()
		if _, err := s.queue.InsertTx(ctx, tx.Raw(), args, &opts); err != nil {
			return fmt.Errorf("enqueue location cluster resolution: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit location projection publish: %w", err)
	}
	return nil
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
	settingsRow, err := s.queries.GetSettings(ctx)
	if err != nil {
		return 0, fmt.Errorf("load geocoding settings: %w", err)
	}
	geocoding, err := normalizeStoredGeocoding(settingsRow)
	if err != nil {
		return 0, fmt.Errorf("load geocoding settings: %w", err)
	}
	if settingsRow.GeocodingRevision != geocodingRevision || !geocoding.IsEnabled() {
		return 0, nil
	}

	now := time.Now().UTC()
	clusters, err := s.queries.ListPendingLocationClusters(ctx, repo.ListPendingLocationClustersParams{
		Now:          dbtypes.NewTimestamp(now),
		RepositoryID: nil,
		OwnerID:      nil,
		Limit:        maxGeocodeClustersPerRun,
	})
	if err != nil {
		return 0, fmt.Errorf("list pending location clusters: %w", err)
	}
	geocoder := newReverseGeocoder(geocoding, s.pacer)
	for _, cluster := range clusters {
		current, err := s.revisionIsCurrent(ctx, geocodingRevision)
		if err != nil {
			return 0, err
		}
		if !current {
			return 0, nil
		}

		cached, cacheErr := s.queries.GetReverseGeocodeCache(ctx, repo.GetReverseGeocodeCacheParams{
			SourceKey: geocoding.SourceKey(),
			Geohash:   cluster.Geohash,
			Provider:  geocoding.Provider,
			Language:  geocoding.Language,
			Now:       dbtypes.NewTimestamp(time.Now().UTC()),
		})
		if cacheErr == nil {
			published, err := s.publishClusterResult(ctx, geocodingRevision, cluster.ClusterID, geocoding.Provider, "cached", cached.Label, cached.Country, cached.Region, cached.City)
			if err != nil {
				return 0, err
			}
			if !published {
				return 0, nil
			}
			continue
		}
		if !errors.Is(cacheErr, sql.ErrNoRows) {
			return 0, fmt.Errorf("get reverse geocode cache: %w", cacheErr)
		}

		current, err = s.revisionIsCurrent(ctx, geocodingRevision)
		if err != nil {
			return 0, err
		}
		if !current {
			return 0, nil
		}
		result, providerErr := geocoder.Reverse(ctx, cluster.CentroidLatitude, cluster.CentroidLongitude)
		if providerErr != nil {
			if ctx.Err() != nil {
				return 0, ctx.Err()
			}
			published, err := s.recordProviderFailure(ctx, geocodingRevision, cluster, providerErr)
			if err != nil {
				return 0, err
			}
			if !published {
				return 0, nil
			}
			continue
		}

		published, err := s.publishRemoteResult(ctx, geocodingRevision, geocoding, cluster, result)
		if err != nil {
			return 0, err
		}
		if !published {
			return 0, nil
		}
	}

	current, err := s.revisionIsCurrent(ctx, geocodingRevision)
	if err != nil {
		return 0, err
	}
	if !current {
		return 0, nil
	}
	schedule, err := s.queries.GetPendingLocationClusterSchedule(ctx)
	if err != nil {
		return 0, fmt.Errorf("schedule pending location clusters: %w", err)
	}
	if schedule.PendingCount == 0 {
		return 0, nil
	}
	nextAttemptAt := scheduleUnixMicro(schedule.NextAttemptAt)
	if nextAttemptAt <= 0 {
		return time.Second, nil
	}
	delay := time.Until(time.UnixMicro(nextAttemptAt))
	if delay < time.Second {
		return time.Second, nil
	}
	return delay, nil
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

func (s *locationService) revisionIsCurrent(ctx context.Context, revision int64) (bool, error) {
	row, err := s.queries.GetSettings(ctx)
	if err != nil {
		return false, fmt.Errorf("check geocoding revision: %w", err)
	}
	return row.GeocodingRevision == revision, nil
}

func (s *locationService) publishClusterResult(ctx context.Context, revision int64, clusterID uuid.UUID, provider, status string, label, country, region, city *string) (bool, error) {
	rows, err := s.queries.UpdateLocationClusterGeocodeIfRevision(ctx, repo.UpdateLocationClusterGeocodeIfRevisionParams{
		Label:                label,
		Country:              country,
		Region:               region,
		City:                 city,
		Provider:             &provider,
		GeocodeStatus:        status,
		GeocodedAt:           dbtypes.NewTimestamp(time.Now().UTC()),
		GeocodeAttemptCount:  0,
		GeocodeNextAttemptAt: dbtypes.Timestamp{},
		ClusterID:            clusterID,
		GeocodingRevision:    revision,
	})
	if err != nil {
		return false, fmt.Errorf("publish location cluster result: %w", err)
	}
	return rows > 0, nil
}

func (s *locationService) publishRemoteResult(ctx context.Context, revision int64, geocoding settings.Geocoding, cluster repo.LocationCluster, result ReverseGeocodeResult) (bool, error) {
	tx, err := s.writer.BeginTx(ctx, catalogtx.OperationLocationRemotePublish, nil)
	if err != nil {
		return false, fmt.Errorf("begin reverse geocode publication: %w", err)
	}
	defer tx.Rollback()
	qtx := s.queries.WithTx(tx.Raw())
	current, err := qtx.GetSettings(ctx)
	if err != nil {
		return false, fmt.Errorf("check geocoding revision before publication: %w", err)
	}
	if current.GeocodingRevision != revision {
		return false, nil
	}
	cache, err := qtx.UpsertReverseGeocodeCache(ctx, repo.UpsertReverseGeocodeCacheParams{
		SourceKey:   geocoding.SourceKey(),
		Geohash:     cluster.Geohash,
		Provider:    geocoding.Provider,
		Language:    geocoding.Language,
		Latitude:    cluster.CentroidLatitude,
		Longitude:   cluster.CentroidLongitude,
		Label:       result.Label,
		Country:     result.Country,
		Region:      result.Region,
		City:        result.City,
		RawResponse: dbtypes.JSON(result.RawResponse),
		QueriedAt:   dbtypes.NewTimestamp(time.Now().UTC()),
		ExpiresAt:   dbtypes.NewTimestamp(time.Now().UTC().Add(reverseGeocodeCacheTTL)),
	})
	if err != nil {
		return false, fmt.Errorf("cache reverse geocode result: %w", err)
	}
	rows, err := qtx.UpdateLocationClusterGeocodeIfRevision(ctx, repo.UpdateLocationClusterGeocodeIfRevisionParams{
		Label:                cache.Label,
		Country:              cache.Country,
		Region:               cache.Region,
		City:                 cache.City,
		Provider:             &geocoding.Provider,
		GeocodeStatus:        "resolved",
		GeocodedAt:           dbtypes.NewTimestamp(time.Now().UTC()),
		GeocodeAttemptCount:  0,
		GeocodeNextAttemptAt: dbtypes.Timestamp{},
		ClusterID:            cluster.ClusterID,
		GeocodingRevision:    revision,
	})
	if err != nil {
		return false, fmt.Errorf("publish location cluster result: %w", err)
	}
	if rows == 0 {
		return false, nil
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("commit reverse geocode publication: %w", err)
	}
	return true, nil
}

func (s *locationService) recordProviderFailure(ctx context.Context, revision int64, cluster repo.LocationCluster, providerErr error) (bool, error) {
	var typed *geocodeProviderError
	if !errors.As(providerErr, &typed) {
		typed = &geocodeProviderError{retryable: true, cause: providerErr}
	}
	attempt := cluster.GeocodeAttemptCount + 1
	status := "pending"
	nextAttemptAt := dbtypes.Timestamp{}
	if !typed.retryable || attempt >= maxProviderAttempts {
		status = "failed"
	} else {
		nextAttemptAt = dbtypes.NewTimestamp(time.Now().UTC().Add(providerRetryDelay(attempt, typed.retryAfter)))
	}
	rows, err := s.queries.UpdateLocationClusterRetryIfRevision(ctx, repo.UpdateLocationClusterRetryIfRevisionParams{
		GeocodeStatus:        status,
		GeocodeAttemptCount:  attempt,
		GeocodeNextAttemptAt: nextAttemptAt,
		GeocodedAt:           dbtypes.NewTimestamp(time.Now().UTC()),
		ClusterID:            cluster.ClusterID,
		GeocodingRevision:    revision,
	})
	if err != nil {
		return false, fmt.Errorf("persist reverse geocode retry state: %w", err)
	}
	return rows > 0, nil
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
