package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"server/config"
	"server/internal/models"
	"server/internal/queue"
	"server/internal/repository/gorm_repo"
	"server/internal/service"
	"server/internal/storage"
	"server/internal/utils"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func init() {
	log.SetOutput(os.Stdout)

	// Try to load .env file but continue if it's not found
	if err := godotenv.Load(); err != nil {
		log.Println("Running without .env file, using environment variables")
	} else {
		log.Println("Environment variables loaded from .env file")
	}
}

// 该Worker主程序，只且仅只负责处理重负载任务，如图像处理，哈希算法，机器学习微服务调用, gRPC客户端
func main() {
	log.Println("Starting worker service...")

	// Load configuration
	dbConfig := config.LoadDBConfig()

	// Connect to the database
	// 数据库服务应该由API层启动，worker直接链接
	database := gorm_repo.InitDB(dbConfig)

	// Defer closing the database connection
	sqlDB, err := database.DB()
	if err != nil {
		panic(err)
	}
	defer func(sqlDB *sql.DB) {
		err := sqlDB.Close()
		if err != nil {
			panic(err)
		}
	}(sqlDB)

	// Initialize repositories
	// 构建数据库通道
	assetRepo := gorm_repo.NewAssetRepository(database)
	tagRepo := gorm_repo.NewTagRepository(database)

	// Load storage configuration
	storageConfig := storage.LoadStorageConfigFromEnv()
	log.Printf("Using storage strategy: %s (%s)", storageConfig.Strategy, storageConfig.Strategy.GetDescription())
	log.Printf("Storage base path: %s", storageConfig.BasePath)

	// Initialize storage service
	storageService, err := storage.NewStorageWithConfig(storageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize storage service: %v", err)
	}

	// Initialize asset service
	assetService := service.NewAssetService(assetRepo, tagRepo, storageService)

	// Get ML service address from environment or use default
	mlServiceAddr := os.Getenv("ML_SERVICE_ADDR")
	if mlServiceAddr == "" {
		mlServiceAddr = "ml:50051" // Default Docker service name and port
	}
	log.Printf("Connecting to ML service at: %s", mlServiceAddr)

	mlService, err := service.NewMLClient(mlServiceAddr)
	if err != nil {
		log.Fatalf("Failed to connect to ML gRPC server: %v", err)
	}

	// Initialize asset processor
	// 该实例只能，也只应该在worker端创建
	assetProcessor := utils.NewAssetProcessor(assetService, storageService, storageConfig.BasePath, mlService)

	// Initialize task queue
	queueDir := os.Getenv("QUEUE_DIR")
	if queueDir == "" {
		queueDir = "/app/queue" // 持久化队列
	}
	log.Printf("Using queue directory: %s", queueDir)

	taskQueue, err := queue.NewTaskQueue(queueDir, 100) // 缓冲区可以根据需要调整
	if err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}
	defer taskQueue.Close()

	if err := taskQueue.Initialize(); err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}

	// --- Worker Pool 启动部分 ---
	// 根据CPU核心数设置worker数量，这是一个常见的实践
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2 // 保证至少有2个worker
	}
	log.Printf("Starting %d worker(s)...", numWorkers)

	stopChan := make(chan struct{})
	for i := 0; i < numWorkers; i++ {
		workerID := i + 1
		// 为每个worker启动一个goroutine
		go processTasksLoop(workerID, taskQueue, assetService, assetProcessor, storageConfig.BasePath, stopChan)
	}
	// --- Worker Pool 启动部分结束 ---

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping all workers...")

	// 关闭stopChan会通知所有的worker goroutine停止
	close(stopChan)

	// 给予一小段时间让正在处理的任务完成
	time.Sleep(3 * time.Second)
	log.Println("Worker service stopped")
}

// processTasksLoop continuously processes tasks from the queue
// processTasksLoop 接收一个 workerID 用于日志记录
func processTasksLoop(workerID int, taskQueue *queue.TaskQueue, assetService service.AssetService,
	assetProcessor *utils.AssetProcessor, storagePath string, stopChan chan struct{}) {

	log.Printf("[Worker %d] Started, waiting for tasks...", workerID)

	// 每个worker独立运行，但清理任务只需要一个worker执行，
	// 实际生产中可以将清理逻辑移到main或一个单独的goroutine中。
	// 这里为简单起见，让每个worker都可能触发，但由于时间间隔长，影响不大。
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-stopChan:
			log.Printf("[Worker %d] Stopping...", workerID)
			return

		case <-cleanupTicker.C:
			// 理论上只有一个worker需要做这件事
			if workerID == 1 {
				log.Printf("[Worker %d] Running task queue cleanup...", workerID)
				if err := taskQueue.CleanupProcessedTasks(); err != nil {
					log.Printf("[Worker %d] Error cleaning up processed tasks: %v", workerID, err)
				}
			}

		default:
			task, ok := taskQueue.GetTask()
			if !ok {
				// Channel is closed, or more likely, temporarily empty.
				// stopChan会处理关闭逻辑，所以这里只需要等待
				time.Sleep(100 * time.Millisecond)
				continue
			}

			log.Printf("[Worker %d] Picked up task %s (Type: %s)", workerID, task.TaskID, task.Type)

			switch task.Type {
			case string(queue.TaskTypeUpload):
				procTask := task
				procTask.Type = string(queue.TaskTypeProcess)
				if err := taskQueue.EnqueueTask(procTask); err != nil {
					log.Printf("[Worker %d][%s] Enqueue PROCESS failed: %v", workerID, task.TaskID, err)
				} else {
					log.Printf("[Worker %d][%s] UPLOAD completed, queued for PROCESS", workerID, task.TaskID)
				}
				// 无论转换是否成功，都将原始UPLOAD任务标记为完成，防止僵尸
				if err := taskQueue.MarkTaskComplete(task.TaskID); err != nil {
					log.Printf("[Worker %d][%s] Mark UPLOAD complete failed: %v", workerID, task.TaskID, err)
				}

			case string(queue.TaskTypeProcess):
				var asset *models.Asset
				var err error

				if strings.Contains(task.StagedPath, "staging") || strings.Contains(task.StagedPath, "temp") {
					asset, err = assetProcessor.ProcessNewAsset(task.StagedPath, task.UserID, task.FileName)
				} else {
					asset, err = assetProcessor.ProcessExistingAsset(task.StagedPath, task.UserID, task.FileName)
				}

				if err != nil {
					log.Printf("[Worker %d][%s] PROCESS failed: %v", workerID, task.TaskID, err)
					// 即使失败也要标记为完成，防止无限重试
					if errMark := taskQueue.MarkTaskComplete(task.TaskID); errMark != nil {
						log.Printf("[Worker %d][%s] Mark FAILED PROCESS complete failed: %v", workerID, task.TaskID, errMark)
					}
					break // 结束当前任务的处理
				}

				idxTask := task
				idxTask.Type = string(queue.TaskTypeIndex)
				idxTask.ClientHash = asset.Hash
				if err := taskQueue.EnqueueTask(idxTask); err != nil {
					log.Printf("[Worker %d][%s] Enqueue INDEX failed: %v", workerID, task.TaskID, err)
				} else {
					log.Printf("[Worker %d][%s] PROCESS completed, queued for INDEX", workerID, task.TaskID)
				}
				// 标记PROCESS任务完成
				if err := taskQueue.MarkTaskComplete(task.TaskID); err != nil {
					log.Printf("[Worker %d][%s] Mark PROCESS complete failed: %v", workerID, task.TaskID, err)
				}

			case string(queue.TaskTypeIndex):
				if err := assetService.SaveAssetIndex(context.Background(), task.TaskID, task.ClientHash); err != nil {
					log.Printf("[Worker %d][%s] INDEX failed: %v", workerID, task.TaskID, err)
					// 即使失败也要标记为完成
					if errMark := taskQueue.MarkTaskComplete(task.TaskID); errMark != nil {
						log.Printf("[Worker %d][%s] Mark FAILED INDEX complete failed: %v", workerID, task.TaskID, errMark)
					}
					break // 结束当前任务的处理
				}

				if err := taskQueue.MarkTaskComplete(task.TaskID); err != nil {
					log.Printf("[Worker %d][%s] Mark INDEX complete failed: %v", workerID, task.TaskID, err)
				} else {
					log.Printf("[Worker %d][%s] INDEX done.", workerID, task.TaskID)
				}

			case string(queue.TaskTypeScan):
				log.Printf("[Worker %d][%s] SCAN folder: %s", workerID, task.TaskID, task.StagedPath)
				files, err := utils.ListImagesInDir(task.StagedPath)
				if err != nil {
					log.Printf("[Worker %d][%s] Scan failed: %v", workerID, task.TaskID, err)
				} else {
					for _, imgPath := range files {
						newTask := queue.Task{
							TaskID:     uuid.New().String(),
							Type:       string(queue.TaskTypeProcess),
							StagedPath: imgPath,
							UserID:     task.UserID,
							Timestamp:  time.Now(),
							FileName:   filepath.Base(imgPath),
						}
						if err := taskQueue.EnqueueTask(newTask); err != nil {
							log.Printf("[Worker %d][%s] Enqueue process for %s failed: %v",
								workerID, task.TaskID, imgPath, err)
						}
					}
				}
				// 标记SCAN任务完成
				if err := taskQueue.MarkTaskComplete(task.TaskID); err != nil {
					log.Printf("[Worker %d][%s] mark SCAN complete failed: %v", workerID, task.TaskID, err)
				}

			default:
				log.Printf("[Worker %d][%s] Unknown task type %q, marking as complete to avoid blocking queue", workerID, task.TaskID, task.Type)
				// 对未知类型的任务也标记为完成，防止队列阻塞
				if err := taskQueue.MarkTaskComplete(task.TaskID); err != nil {
					log.Printf("[Worker %d][%s] Mark UNKNOWN complete failed: %v", workerID, task.TaskID, err)
				}
			}
		}
	}
}
