package concurrent

import (
	"context"
	"errors"
	"sync"
)

//通用任务协程池

var ErrExexcutorCapacity = errors.New("协程池容量不合法")
var ErrExexcutorClosed = errors.New("协程池已关闭")

type Task interface {
	Handle()
}

type Executor struct {
	sync.Mutex
	capacity int
	active   int
	workers  chan *worker
	ctx      context.Context
	cancel   context.CancelFunc
	closed   bool
}

func (e *Executor) IsClose() bool {
	return e.closed
}
func NewExecutor(capacity int) *Executor {
	if capacity <= 0 {
		capacity = 1
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Executor{
		capacity: capacity,
		workers:  make(chan *worker, capacity),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (e *Executor) getWorker() *worker {
	select {
	case w := <-e.workers:
		return w
	default:
	}
	if e.active < e.capacity {
		e.active++
		w := &worker{
			e:        e,
			taskChan: make(chan func()),
		}
		w.run()
		return w
	}
	return <-e.workers
}
func (e *Executor) recoverWorker(w *worker) {
	e.workers <- w
}

func (e *Executor) Submit(task func()) error {
	if e.closed {
		return ErrExexcutorClosed
	}
	w := e.getWorker()
	w.taskChan <- task
	return nil
}

func (e *Executor) SubmitSyncBatch(tasks []func()) (err error) {
	var wg sync.WaitGroup
	wg.Add(len(tasks))

	for _, t := range tasks {
		cb := t
		err = e.Submit(func() {
			cb()
			wg.Done()
		})
		if err != nil {
			wg.Done()
		}
	}
	wg.Wait()
	return
}

func (e *Executor) Close() {
	e.closed = true
	e.cancel()
}

type worker struct {
	e        *Executor
	taskChan chan func()
}

func (w *worker) run() {
	go func() {
		for {
			select {
			case <-w.e.ctx.Done():
				return
			case task := <-w.taskChan:
				if task != nil {
					task()
				}
				w.e.recoverWorker(w)
			}
		}
	}()
}
