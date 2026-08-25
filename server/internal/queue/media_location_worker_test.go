package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/riverqueue/river"
)

type locationClusterServiceStub struct {
	moreWork   bool
	err        error
	repository *string
	owner      *int32
}

func (s *locationClusterServiceStub) RebuildLocationClusters(
	_ context.Context,
	repositoryID *string,
	ownerID *int32,
) (bool, error) {
	s.repository = repositoryID
	s.owner = ownerID
	return s.moreWork, s.err
}

func TestRebuildLocationClustersWorkerSnoozesBoundedContinuation(t *testing.T) {
	t.Parallel()

	repositoryID := "11111111-1111-4111-8111-111111111111"
	ownerID := int32(7)
	service := &locationClusterServiceStub{moreWork: true}
	worker := &RebuildLocationClustersWorker{LocationService: service}

	err := worker.Work(context.Background(), &river.Job[RebuildLocationClustersArgs]{
		Args: RebuildLocationClustersArgs{RepositoryID: &repositoryID, OwnerID: &ownerID},
	})
	var snooze *river.JobSnoozeError
	if !errors.As(err, &snooze) {
		t.Fatalf("worker error = %v, want River snooze", err)
	}
	if service.repository == nil || *service.repository != repositoryID ||
		service.owner == nil || *service.owner != ownerID {
		t.Fatalf("service scope = %v/%v, want %s/%d", service.repository, service.owner, repositoryID, ownerID)
	}
}

func TestRebuildLocationClustersWorkerCompletesConvergedRevision(t *testing.T) {
	t.Parallel()

	service := &locationClusterServiceStub{}
	worker := &RebuildLocationClustersWorker{LocationService: service}
	if err := worker.Work(context.Background(), &river.Job[RebuildLocationClustersArgs]{}); err != nil {
		t.Fatalf("converged location projection: %v", err)
	}
}
