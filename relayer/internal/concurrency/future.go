package concurrency

import (
	"sync"
	"time"
)

type Future[T any] struct {
	result T
	err    error
	done   chan struct{}
	once   sync.Once
}

func NewFuture[T any](fn func() (T, error)) *Future[T] {
	f := &Future[T]{done: make(chan struct{})}
	go func() {
		defer close(f.done)
		f.result, f.err = fn()
	}()
	return f
}

func (f *Future[T]) Get() (T, error) {
	<-f.done
	return f.result, f.err
}

func (f *Future[T]) GetWithTimeout(timeout time.Duration) (T, error) {
	select {
	case <-f.done:
		return f.result, f.err
	case <-time.After(timeout):
		var zero T
		return zero, ErrFutureTimeout
	}
}

func (f *Future[T]) IsDone() bool {
	select {
	case <-f.done:
		return true
	default:
		return false
	}
}

var ErrFutureTimeout = &FutureError{msg: "future timeout"}

type FutureError struct {
	msg string
}

func (e *FutureError) Error() string { return e.msg }

type FutureGroup[T any] struct {
	futures []*Future[T]
}

func NewFutureGroup[T any]() *FutureGroup[T] {
	return &FutureGroup[T]{}
}

func (fg *FutureGroup[T]) Submit(fn func() (T, error)) {
	fg.futures = append(fg.futures, NewFuture(fn))
}

func (fg *FutureGroup[T]) WaitAll() []T {
	results := make([]T, len(fg.futures))
	for i, f := range fg.futures {
		results[i], _ = f.Get()
	}
	return results
}

func (fg *FutureGroup[T]) WaitAny(timeout time.Duration) (int, T, error) {
	type result struct {
		index int
		value T
	}
	ch := make(chan result, len(fg.futures))
	for i, f := range fg.futures {
		go func(idx int, fut *Future[T]) {
			val, err := fut.Get()
			if err == nil {
				ch <- result{idx, val}
			}
		}(i, f)
	}
	select {
	case r := <-ch:
		return r.index, r.value, nil
	case <-time.After(timeout):
		var zero T
		return -1, zero, ErrFutureTimeout
	}
}
