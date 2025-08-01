package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// RiverQueue 是对业务 Queue[T] 接口的 River 实现
type RiverQueue[T any] struct {
	dbPool       *pgxpool.Pool
	workers      *river.Workers
	queueConfigs map[string]river.QueueConfig
	client       *river.Client[pgx.Tx]
}

// NewRiverQueue 用于初始化队列适配器
func NewRiverQueue[T any](dbPool *pgxpool.Pool) *RiverQueue[T] {
	return &RiverQueue[T]{
		dbPool:       dbPool,
		workers:      river.NewWorkers(),
		queueConfigs: make(map[string]river.QueueConfig),
	}
}

// Enqueue 立即插入作业
func (r *RiverQueue[T]) Enqueue(ctx context.Context, jobType string, payload T) (string, error) {
	// 使用默认 Insert，不在事务中
	result, err := r.client.Insert(ctx,
		PayloadArgs[T]{Data: payload, kind: jobType},
		nil,
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", result.Job.ID), nil
}

// EnqueueIn 延迟插入作业
func (r *RiverQueue[T]) EnqueueIn(ctx context.Context, jobType string, payload T, delay time.Duration) (string, error) {
	opts := &river.InsertOpts{ScheduledAt: time.Now().Add(delay)}
	result, err := r.client.Insert(ctx,
		PayloadArgs[T]{Data: payload, kind: jobType},
		opts,
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", result.Job.ID), nil
}

// EnqueueTx 在给定事务中插入作业
func (r *RiverQueue[T]) EnqueueTx(ctx context.Context, tx any, jobType string, payload T) (string, error) {
	result, err := r.client.InsertTx(ctx,
		tx.(pgx.Tx),
		PayloadArgs[T]{Data: payload, kind: jobType},
		nil,
	)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", result.Job.ID), nil
}

// RegisterWorker
func (r *RiverQueue[T]) RegisterWorker(
	jobType JobType,
	opts WorkerOptions,
	handler func(ctx context.Context, job Job[T]) error,
) {
	r.queueConfigs[string(jobType)] = river.QueueConfig{MaxWorkers: opts.Concurrency}
	w := &genericWorker[T]{handler: handler}
	river.AddWorker(r.workers, w)
}

func (r *RiverQueue[T]) Start(ctx context.Context) error {
	cfg := &river.Config{
		Queues:  r.queueConfigs,
		Workers: r.workers,
	}
	cli, err := river.NewClient(riverpgxv5.New(r.dbPool), cfg)
	if err != nil {
		return err
	}
	r.client = cli
	return r.client.Start(ctx)
}

func (r *RiverQueue[T]) Stop(ctx context.Context) error {
	return r.client.Stop(ctx)
}
