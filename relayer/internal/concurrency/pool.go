package concurrency

import (
	"context"
	"sync"
)

type WorkerPool struct {
	jobs    chan func()
	workers int
	wg      sync.WaitGroup
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewWorkerPool(workers, queueSize int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		jobs:    make(chan func(), queueSize),
		workers: workers,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (wp *WorkerPool) Start() {
	for i := 0; i < wp.workers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()
	for {
		select {
		case <-wp.ctx.Done():
			return
		case job, ok := <-wp.jobs:
			if !ok {
				return
			}
			job()
		}
	}
}

func (wp *WorkerPool) Submit(job func()) {
	wp.jobs <- job
}

func (wp *WorkerPool) SubmitWithContext(ctx context.Context, job func()) error {
	select {
	case wp.jobs <- job:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
	close(wp.jobs)
}

func (wp *WorkerPool) QueueSize() int {
	return len(wp.jobs)
}

// FanOut distributes work across multiple workers.
func FanOut(ctx context.Context, input <-chan interface{}, workerCount int, fn func(interface{}) interface{}) []interface{} {
	outputs := make([]chan interface{}, workerCount)
	for i := range outputs {
		outputs[i] = make(chan interface{})
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(ch chan interface{}) {
			defer wg.Done()
			defer close(ch)
			for item := range input {
				result := fn(item)
				select {
				case ch <- result:
				case <-ctx.Done():
					return
				}
			}
		}(outputs[i])
	}

	merged := make(chan interface{})
	go func() {
		wg.Wait()
		close(merged)
	}()

	go func() {
		var inner sync.WaitGroup
		inner.Add(len(outputs))
		for _, ch := range outputs {
			go func(c chan interface{}) {
				defer inner.Done()
				for item := range c {
					select {
					case merged <- item:
					case <-ctx.Done():
						return
					}
				}
			}(ch)
		}
		inner.Wait()
		close(merged)
	}()

	var results []interface{}
	for item := range merged {
		results = append(results, item)
	}
	return results
}

// FanIn merges multiple channels into one.
func FanIn(ctx context.Context, channels ...<-chan interface{}) <-chan interface{} {
	merged := make(chan interface{})
	var wg sync.WaitGroup
	for _, ch := range channels {
		wg.Add(1)
		go func(c <-chan interface{}) {
			defer wg.Done()
			for item := range c {
				select {
				case merged <- item:
				case <-ctx.Done():
					return
				}
			}
		}(ch)
	}
	go func() {
		wg.Wait()
		close(merged)
	}()
	return merged
}

// Pipeline chains processing stages.
type Pipeline struct {
	stages []func(<-chan interface{}) <-chan interface{}
}

func NewPipeline() *Pipeline {
	return &Pipeline{}
}

func (p *Pipeline) AddStage(stage func(<-chan interface{}) <-chan interface{}) *Pipeline {
	p.stages = append(p.stages, stage)
	return p
}

func (p *Pipeline) Run(ctx context.Context, input <-chan interface{}) <-chan interface{} {
	current := input
	for _, stage := range p.stages {
		current = stage(current)
	}
	return current
}
