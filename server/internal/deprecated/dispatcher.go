package deprecated

// import (
// 	"context"
// 	"log"
// 	taskqueuewal "server/internal/deprecated/task_queue_wal"
// 	"server/internal/processors"
// 	"sync"
// )

// type ProcessorPool struct {
// 	Tasks       []*taskqueuewal.Task
// 	concurrency int
// 	tasksChan   chan *taskqueuewal.Task
// 	wg          sync.WaitGroup
// 	ap          *processors.AssetProcessor
// }

// func (p *ProcessorPool) worker(ctx context.Context) {
// 	for task := range p.tasksChan {
// 		_, err := p.ap.ProcessAsset(ctx, *task)
// 		if err != nil {
// 			// TODO: handle errors, we can implement finalized layer here
// 			log.Print("error")
// 		}
// 		p.wg.Done()
// 	}
// }

// func (p *ProcessorPool) Run(ctx context.Context) {
// 	// Cache
// 	p.tasksChan = make(chan *taskqueuewal.Task, p.concurrency)

// 	for i := 0; i < p.concurrency; i++ {
// 		go p.worker(ctx)
// 	}

// 	p.wg.Add(len(p.Tasks))

// 	for _, task := range p.Tasks {
// 		p.tasksChan <- task
// 	}

// 	close(p.tasksChan)

// 	p.wg.Wait()
// }
