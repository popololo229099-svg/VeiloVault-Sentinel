package concurrency

import (
	"sync"
	"time"
)

type AsyncQueue[T any] struct {
	items  chan T
	mu     sync.Mutex
	closed bool
 onClose func(T)
}

func NewAsyncQueue[T any](bufferSize int) *AsyncQueue[T] {
	return &AsyncQueue[T]{items: make(chan T, bufferSize)}
}

func (q *AsyncQueue[T]) Push(item T) error {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return ErrQueueClosed
	}
	q.mu.Unlock()
	q.items <- item
	return nil
}

func (q *AsyncQueue[T]) PushNonBlocking(item T) bool {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return false
	}
	q.mu.Unlock()
	select {
	case q.items <- item:
		return true
	default:
		return false
	}
}

func (q *AsyncQueue[T]) Pop() (T, bool) {
	item, ok := <-q.items
	return item, ok
}

func (q *AsyncQueue[T]) PopWithTimeout(timeout time.Duration) (T, bool) {
	select {
	case item := <-q.items:
		return item, true
	case <-time.After(timeout):
		var zero T
		return zero, false
	}
}

func (q *AsyncQueue[T]) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.items)
	}
}

func (q *AsyncQueue[T]) Len() int {
	return len(q.items)
}

type ProducerConsumer[T any] struct {
	queue     *AsyncQueue[T]
	producers int
	consumers int
	handler   func(T)
	wg        sync.WaitGroup
}

func NewProducerConsumer[T any](bufferSize, producers, consumers int, handler func(T)) *ProducerConsumer[T] {
	return &ProducerConsumer[T]{
		queue:     NewAsyncQueue[T](bufferSize),
		producers: producers,
		consumers: consumers,
		handler:   handler,
	}
}

func (pc *ProducerConsumer[T]) Start(produce func() (T, bool)) {
	for i := 0; i < pc.consumers; i++ {
		pc.wg.Add(1)
		go pc.consume()
	}
	for i := 0; i < pc.producers; i++ {
		pc.wg.Add(1)
		go pc.produce(produce)
	}
}

func (pc *ProducerConsumer[T]) produce(fn func() (T, bool)) {
	defer pc.wg.Done()
	for {
		item, ok := fn()
		if !ok {
			pc.queue.Close()
			return
		}
		pc.queue.Push(item)
	}
}

func (pc *ProducerConsumer[T]) consume() {
	defer pc.wg.Done()
	for item := range pc.queue.items {
		pc.handler(item)
	}
}

func (pc *ProducerConsumer[T]) Wait() {
	pc.wg.Wait()
}

var ErrQueueClosed = &QueueError{"queue closed"}

type QueueError struct{ msg string }

func (e *QueueError) Error() string { return e.msg }
