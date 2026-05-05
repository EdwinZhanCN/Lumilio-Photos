package scanner

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Scheduler struct {
	scanner *Scanner
	logger  *zap.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewScheduler(scanner *Scanner, logger *zap.Logger) *Scheduler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Scheduler{
		scanner: scanner,
		logger:  logger.With(zap.String("component", "repository_scan_scheduler")),
	}
}

func (s *Scheduler) Start(ctx context.Context) error {
	if s == nil || s.scanner == nil {
		return nil
	}
	if !s.scanner.cfg.Enabled {
		s.logger.Info("repository scan scheduler disabled", zap.String("operation", "repository_scan.start"))
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(runCtx)
	}()
	return nil
}

func (s *Scheduler) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) run(ctx context.Context) {
	interval := time.Duration(s.scanner.cfg.IntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	s.enqueueAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.enqueueAll(ctx)
		}
	}
}

func (s *Scheduler) enqueueAll(ctx context.Context) {
	repositories, err := s.scanner.queries.ListActiveRepositories(ctx)
	if err != nil {
		s.logger.Warn("failed to list active repositories for scan",
			zap.String("operation", "repository_scan.enqueue_all"),
			zap.Error(err),
		)
		return
	}

	sem := make(chan struct{}, s.scanner.cfg.MaxConcurrentRepos)
	var wg sync.WaitGroup
	for _, repository := range repositories {
		repository := repository
		if !repository.RepoID.Valid {
			continue
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := s.scanner.EnqueuePeriodicScan(ctx, repository.RepoID.String()); err != nil {
				s.logger.Warn("failed to enqueue periodic repository scan",
					zap.String("operation", "repository_scan.enqueue"),
					zap.String("repository_id", repository.RepoID.String()),
					zap.Error(err),
				)
			}
		}()
	}
	wg.Wait()
}
