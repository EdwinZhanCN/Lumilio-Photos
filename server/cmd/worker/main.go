package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"path/filepath"
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

	// Initialize services with configurable storage
	assetService, err := service.NewAssetServiceWithConfig(assetRepo, tagRepo, storageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize asset service: %v", err)
	}
	mlService, err := service.NewMLClient("localhost:50051")
	if err != nil {
		log.Fatalf("Failed to connect to ML gRPC server: %v", err)
	}

	// Initialize asset processor
	// 该实例只能，也只应该在worker端创建
	assetProcessor := utils.NewAssetProcessor(assetService, nil, storageConfig.BasePath, mlService)

	// Initialize task queue
	queueDir := os.Getenv("QUEUE_DIR")
	if queueDir == "" {
		queueDir = "/app/queue" // 持久化队列
	}
	log.Printf("Using queue directory: %s", queueDir)

	taskQueue, err := queue.NewTaskQueue(queueDir, 100)
	if err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}
	defer taskQueue.Close()

	// Initialize task queue and start processing
	if err := taskQueue.Initialize(); err != nil {
		log.Fatalf("Failed to initialize task queue: %v", err)
	}

	// Start the task processor
	stopChan := make(chan struct{})
	go processTasksLoop(taskQueue, assetService, assetProcessor, storageConfig.BasePath, stopChan)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Shutdown signal received, stopping worker...")
	close(stopChan)
	// Give tasks some time to complete
	time.Sleep(2 * time.Second)
	log.Println("Worker service stopped")
}

// processTasksLoop continuously processes tasks from the queue
func processTasksLoop(taskQueue *queue.TaskQueue, assetService service.AssetService,
	assetProcessor *utils.AssetProcessor, storagePath string, stopChan chan struct{}) {
	log.Println("Task processor started, waiting for tasks...")

	// Create cleanup ticker
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-stopChan:
			log.Println("Task processor stopping...")
			return

		case <-cleanupTicker.C:
			log.Println("Running task queue cleanup...")
			if err := taskQueue.CleanupProcessedTasks(); err != nil {
				log.Printf("Error cleaning up processed tasks: %v", err)
			}

		default:
			task, ok := taskQueue.GetTask()
			if !ok {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			switch task.Type {
			case queue.TaskTypeUpload:
				// 1) UPLOAD → PROCESS
				procTask := task
				procTask.Type = queue.TaskTypeProcess
				if err := taskQueue.EnqueueTask(procTask); err != nil {
					log.Printf("[%s] enqueue PROCESS failed: %v", task.TaskID, err)
				}

			case queue.TaskTypeProcess:
				// 2) PROCESS → INDEX
				// Determine if this is from UPLOAD (staged file) or SCAN (existing file)
				var asset *models.Asset
				var err error

				// Check if the file path indicates it's in staging area
				if strings.Contains(task.StagedPath, "staging") || strings.Contains(task.StagedPath, "temp") {
					// This is from UPLOAD task - process staged file
					log.Printf("[%s] PROCESS from UPLOAD: %s", task.TaskID, task.StagedPath)
					asset, err = assetProcessor.ProcessNewAsset(task.StagedPath, task.UserID, task.FileName)
				} else {
					// This is from SCAN task - process existing file
					log.Printf("[%s] PROCESS from SCAN: %s", task.TaskID, task.StagedPath)
					asset, err = assetProcessor.ProcessExistingAsset(task.StagedPath, task.UserID, task.FileName)
				}

				if err != nil {
					log.Printf("[%s] PROCESS failed: %v", task.TaskID, err)
					break
				}

				idxTask := task
				idxTask.Type = queue.TaskTypeIndex
				idxTask.ClientHash = asset.Hash // 把最终 hash 透传给 INDEX
				if err := taskQueue.EnqueueTask(idxTask); err != nil {
					log.Printf("[%s] enqueue INDEX failed: %v", task.TaskID, err)
				} else {
					log.Printf("[%s] PROCESS completed, queued for INDEX", task.TaskID)
				}

			case queue.TaskTypeIndex:
				// 3) INDEX → 写入数据库并标记完成
				if err := assetService.SaveAssetIndex(
					context.Background(), task.TaskID, task.ClientHash,
				); err != nil {
					log.Printf("[%s] INDEX failed: %v", task.TaskID, err)
					break
				}
				if err := taskQueue.MarkTaskComplete(task.TaskID); err != nil {
					log.Printf("[%s] mark complete failed: %v", task.TaskID, err)
				} else {
					log.Printf("[%s] INDEX done", task.TaskID)
				}

			case queue.TaskTypeScan:
				// 1) SCAN -> PROCESS
				log.Printf("[%s] SCAN folder: %s", task.TaskID, task.StagedPath)
				files, err := utils.ListImagesInDir(task.StagedPath)
				if err != nil {
					log.Printf("[%s] scan failed: %v", task.TaskID, err)
				} else {
					for _, imgPath := range files {
						newTask := queue.Task{
							TaskID:     uuid.New().String(),
							Type:       queue.TaskTypeProcess,
							StagedPath: imgPath,
							UserID:     task.UserID,
							Timestamp:  time.Now(),
							FileName:   filepath.Base(imgPath),
						}
						if err := taskQueue.EnqueueTask(newTask); err != nil {
							log.Printf("[%s] enqueue process for %s failed: %v",
								task.TaskID, imgPath, err)
						}
					}
				}
				// 自己标记为完成
				if err := taskQueue.MarkTaskComplete(task.TaskID); err != nil {
					log.Printf("[%s] mark SCAN complete failed: %v", task.TaskID, err)
				}

			default:
				log.Printf("[%s] unknown task type %q", task.TaskID, task.Type)
			}
		}
	}
}
