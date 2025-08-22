package queue

import (
	"context"
	"time"
)

type Job[T any] interface {
	ID() string
	Type() JobType
	Payload() T
	Attempt() int
}

type JobType string

// 定义系统中所有已知的作业类型
const (
	// JobTypeProcessAsset 是处理新上传或发现的资产的核心任务。
	JobTypeProcessAsset JobType = "process_asset"

	// JobTypeCalcPHash 是一个计算感知哈希的重度AI任务。
	// 它可以被 process_asset 任务派生出来。
	JobTypeCalcPHash JobType = "calculate_phash"

	// JobTypeTranscodeVideo 是一个耗时的视频转码任务。
	// 用于为“普通模式”的视频创建Web兼容版本。
	JobTypeTranscodeVideo JobType = "transcode_video"

	// JobTypeScanLibrary 是一个周期性任务，用于扫描媒体库文件夹以发现新文件。
	JobTypeScanLibrary JobType = "scan_library"

	// JobTypeCleanup 是一个周期性任务，用于清理临时文件等维护工作。
	JobTypeCleanup JobType = "cleanup_tasks"

	// JobCLIPProcess 是一个通过gRPC与ML微服务通信并获取图片Embedding的工作。
	JobCLIPProcess JobType = "process_clip"
)

type RetryPolicy struct {
	MaxRetries   int           // 最大重试次数, e.g., 10
	InitialDelay time.Duration // 初始延迟, e.g., 5 * time.Second
	MaxDelay     time.Duration // 最大延迟上限, e.g., 30 * time.Minute
	UseDLQ       bool          // 是否使用死信队列
}

// WorkerOptions 配置消费者并发和重试策略
type WorkerOptions struct {
	Concurrency int
	Policy      RetryPolicy
}

type Queue[T any] interface {
	// Enqueue 将作业立即入队
	Enqueue(ctx context.Context, jobType string, payload T) (jobID string, err error)
	// EnqueueIn 将作业延迟入队
	EnqueueIn(ctx context.Context, jobType string, payload T, delay time.Duration) (jobID string, err error)
	// EnqueueTx 在数据库事务中入队
	EnqueueTx(ctx context.Context, tx any, jobType string, payload T) (jobID string, err error)

	// RegisterWorker 注册消费者处理函数
	RegisterWorker(
		jobType string,
		opts WorkerOptions,
		handler func(ctx context.Context, job Job[T]) error,
	)

	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
