# WAL (Write-Ahead Log) 任务队列实现
在此后端应用中，API层只作为接受文件并落盘的操作，负责创建任务，这里的任务通常是复杂的，Time-Consuming的。对此，我们可以将任务分成几类，或者可以说是为任务的类型进行一个封装。让我们来看`Task`Model的具体实现

```go
type TaskType string

//为任务类型创建封装，用字符串会有歧义
const (
  TaskTypeUpload  TaskType = "UPLOAD"
  TaskTypeScan    TaskType = "SCAN"
  TaskTypeProcess TaskType = "PROCESS"
  TaskTypeIndex   TaskType = "INDEX"
  //...方便以后添加扩展
)

type Task struct {
	TaskID      string    `json:"taskId"`
	Type		TaskType  `json:"type"` 	//任务类型
	ClientHash  string    `json:"clientHash"`
	StagedPath  string    `json:"stagedPath"`
	UserID      string    `json:"userId"`
	Timestamp   time.Time `json:"timestamp"`
	ContentType string    `json:"contentType,omitempty"`
	FileName    string    `json:"fileName,omitempty"`
}
```

这样子，我们就能轻松明了得在API层创建任务。例如，将上传任务，我们标记为TaskTypeUpload

```go
task := queue.Task{
  TaskID:      uuid.New().String(),
  Type:        queue.TaskTypeUpload,    // 新增
  ClientHash:  clientHash,
  StagedPath:  stagingFilePath,
  UserID:      userID,
  Timestamp:   time.Now(),
  ContentType: header.Header.Get("Content-Type"),
  FileName:    header.Filename,
}
```

要深入了解Write-Head Log的文件持久化队列 `TaskQueue` 在内存与磁盘上如何配合工作的，我们先要了解为什么我们需要这个架构。

- 持久化：我们在创建一个任务时，并不是在内存中定义一个变量，结构体，或者指针。而是直接在磁盘下写下`tasks.wal`这个特定的文件。这对该应用作为NAS照片管理应用的意义巨大。WAL确保了每个任务先落盘，再消费，如果进程宕机，重启，Worker能够继续进行任务。Redis 本身是内存型的，虽然也能通过 RDB/AOF 做持久化，但配置复杂度更高，一旦持久化策略没调好，还是可能丢数据或恢复不及时。
- 维护简单，降低依赖：我们不需要额外部署Redis或是消息中间件，维护简单，在容器化环境下更好管理。这里也是因为我已经有4个容器了，我希望克制容器的使用，这有利于保证程序的轻量化设计。
- 内存/磁盘资源取舍：对于大规模的上传高并发场景，例如10000张RAW照片，Redis需要保留整个任务队列，会吃大量RAM，文件WAL把历史队列压在磁盘上，这里也是考虑到NAS作为高存储，低边缘配置的设备，对RAM和磁盘利用率的取舍。

我们还需要了解整个WAL的运作过程。

我们在上述代码定义了入队单元，也就是`Task`任务模型，其是我们对任务元数据的封装。接着我们定义`TaskQueue`模型。任务队列会维护两个文件：

- `tasks.wal`: 所有未完成任务的追加日志（Append-Only）
- `tasks.done`： 已完成的任务的ID表
在内存中维护了一个带缓冲的`chan Task`，大小由`bufferSize`决定。

```go
type TaskQueue struct {
	queueDir    string
	walFile     string
	doneFile    string
	tasks       chan Task
	mutex       sync.Mutex
	bufferSize  int
	initialized bool
}
```

首先，我们将经历初始化，当调用 `Initialize()` 时，会：

1. `os.MkdirAll` 确保队列目录存在
2. 创建（如果不存在）`tasks.wal` 与 `tasks.done`
3. 从 `tasks.done` 加载已完成的 TaskID 到 map
4. 从 `tasks.wal` 扫描所有未完成的任务，非阻塞地入到 `tasks` channel
5. 开一个后台 goroutine 周期性地 `watchForNewTasks`（每秒检查 WAL 文件增长，并将新增行 enqueue）

```go
func (q *TaskQueue) Initialize() error {
  q.mutex.Lock(); defer q.mutex.Unlock()
  if q.initialized { return nil }

  // 确保 WAL、done 文件存在 ……

  completedTasks, _ := q.loadCompletedTaskIDs()
  _ = q.loadExistingTasks(completedTasks)

  go q.watchForNewTasks(completedTasks)
  q.initialized = true
  return nil
}
```

接着，我们需要将API传入的`Task`入队，大致的代码是这样的：
```go
func (q *TaskQueue) EnqueueTask(task Task) error {
  q.mutex.Lock(); defer q.mutex.Unlock()

  // 1) 设置时间戳
  // 2) JSON 序列化
  // 3) 以追加方式写入 tasks.wal（每行一个 Task JSON）
  file, err := os.OpenFile(q.walFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644) // 仅以追加模式打开
  defer file.Close()
  _, err = file.WriteString(string(taskJSON) + "\n") // 写入Task
  return err
}
```
只要有多 goroutine 同时往 WAL（`tasks.wal`）里写，就必须加锁保证写入原子性，否则两条 JSON 可能会互相「插队」，导致日志文件损坏、后续反序列化失败。

然后Worker端的主循环会反复调用 `GetTask()` 拿到一个 `Task` 并根据其 `Type` 分发处理

```go
// GetTask 从 channel 拿任务，channel 里没有则阻塞；返回 ok=false 表示队列已关闭
func (q *TaskQueue) GetTask() (Task, bool) {
  task, ok := <-q.tasks
  return task, ok
}
```

最后，我们需要标记完成和定时清理

```go
func (q *TaskQueue) MarkTaskComplete(taskID string) error {
  // 将 taskID 追加到 tasks.done
}
```
定期（如每日一次）调用 `CleanupProcessedTasks()` 会：

1. 读取 `tasks.done` 构建已完成 ID 的集合
2. 扫描 `tasks.wal`，把仍未完成的任务复制到一个临时文件
3. 用临时文件替换原 `tasks.wal`，并清空 `tasks.done`

这样保证 WAL 只保留未完成的任务。

持续监控新的任务

```go
func (q *TaskQueue) watchForNewTasks(completedTasks map[string]bool) {
  lastSize, _ := q.getFileSize(q.walFile)
  ticker := time.NewTicker(1*time.Second)
  for range ticker.C {
    currentSize, _ := q.getFileSize(q.walFile)
    if currentSize > lastSize {
      // Seek 到 lastSize，读取新增行，反序列化后非阻塞 enqueue
      lastSize = currentSize
    }
    // 并定期刷新 completedTasks map
  }
}
```

保证一旦有新的 `EnqueueTask` 写入 WAL，就能立刻送到 `q.tasks` channel，触发消费。
