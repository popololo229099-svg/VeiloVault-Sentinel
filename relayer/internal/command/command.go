package command

import (
	"fmt"
	"sync"
	"time"
)

type Command interface {
	Execute() error
	Undo() error
	Name() string
}

type CommandInvoker struct {
	history  []Command
	undoList []Command
	mu       sync.RWMutex
}

func NewCommandInvoker() *CommandInvoker {
	return &CommandInvoker{
		history:  make([]Command, 0),
		undoList: make([]Command, 0),
	}
}

func (ci *CommandInvoker) Execute(cmd Command) error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if err := cmd.Execute(); err != nil {
		return err
	}

	ci.history = append(ci.history, cmd)
	ci.undoList = ci.undoList[:0]
	return nil
}

func (ci *CommandInvoker) Undo() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if len(ci.history) == 0 {
		return fmt.Errorf("no commands to undo")
	}

	last := ci.history[len(ci.history)-1]
	ci.history = ci.history[:len(ci.history)-1]

	if err := last.Undo(); err != nil {
		return err
	}

	ci.undoList = append(ci.undoList, last)
	return nil
}

func (ci *CommandInvoker) Redo() error {
	ci.mu.Lock()
	defer ci.mu.Unlock()

	if len(ci.undoList) == 0 {
		return fmt.Errorf("no commands to redo")
	}

	last := ci.undoList[len(ci.undoList)-1]
	ci.undoList = ci.undoList[:len(ci.undoList)-1]

	if err := last.Execute(); err != nil {
		return err
	}

	ci.history = append(ci.history, last)
	return nil
}

func (ci *CommandInvoker) History() []Command {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	result := make([]Command, len(ci.history))
	copy(result, ci.history)
	return result
}

type CommandQueue struct {
	commands chan Command
	workers  int
	mu       sync.RWMutex
	closed   bool
}

func NewCommandQueue(workers, bufferSize int) *CommandQueue {
	if workers <= 0 {
		workers = 1
	}
	if bufferSize <= 0 {
		bufferSize = 100
	}
	cq := &CommandQueue{
		commands: make(chan Command, bufferSize),
		workers:  workers,
	}
	return cq
}

func (cq *CommandQueue) Start(handler func(Command) error) {
	for i := 0; i < cq.workers; i++ {
		go func() {
			for cmd := range cq.commands {
				_ = handler(cmd)
			}
		}()
	}
}

func (cq *CommandQueue) Enqueue(cmd Command) error {
	cq.mu.RLock()
	closed := cq.closed
	cq.mu.RUnlock()

	if closed {
		return fmt.Errorf("queue closed")
	}

	cq.commands <- cmd
	return nil
}

func (cq *CommandQueue) Close() {
	cq.mu.Lock()
	cq.closed = true
	cq.mu.Unlock()
	close(cq.commands)
}

type CommandScheduler struct {
	commands []ScheduledCommand
	mu       sync.RWMutex
}

type ScheduledCommand struct {
	Command   Command
	Schedule  time.Time
	Interval  time.Duration
	Recurring bool
	Active    bool
}

func NewCommandScheduler() *CommandScheduler {
	return &CommandScheduler{
		commands: make([]ScheduledCommand, 0),
	}
}

func (cs *CommandScheduler) Schedule(cmd Command, at time.Time) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.commands = append(cs.commands, ScheduledCommand{
		Command:  cmd,
		Schedule: at,
		Active:   true,
	})
}

func (cs *CommandScheduler) ScheduleRecurring(cmd Command, interval time.Duration) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.commands = append(cs.commands, ScheduledCommand{
		Command:   cmd,
		Schedule:  time.Now().Add(interval),
		Interval:  interval,
		Recurring: true,
		Active:    true,
	})
}

func (cs *CommandScheduler) Cancel(cmd Command) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	for i, sc := range cs.commands {
		if sc.Command == cmd {
			cs.commands[i].Active = false
			break
		}
	}
}

func (cs *CommandScheduler) Due() []Command {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	var due []Command
	now := time.Now()

	for i, sc := range cs.commands {
		if !sc.Active {
			continue
		}
		if now.After(sc.Schedule) || now.Equal(sc.Schedule) {
			due = append(due, sc.Command)
			if sc.Recurring {
				cs.commands[i].Schedule = now.Add(sc.Interval)
			} else {
				cs.commands[i].Active = false
			}
		}
	}

	return due
}

type CommandMiddleware func(Command) Command

type MiddlewareCommand struct {
	inner       Command
	middlewares []CommandMiddleware
	mu          sync.RWMutex
}

func NewMiddlewareCommand(cmd Command, middlewares ...CommandMiddleware) *MiddlewareCommand {
	return &MiddlewareCommand{
		inner:       cmd,
		middlewares: middlewares,
	}
}

func (mc *MiddlewareCommand) Execute() error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	cmd := mc.inner
	for _, mw := range mc.middlewares {
		cmd = mw(cmd)
	}
	return cmd.Execute()
}

func (mc *MiddlewareCommand) Undo() error {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.inner.Undo()
}

func (mc *MiddlewareCommand) Name() string {
	return mc.inner.Name()
}

type CommandValidator interface {
	Validate(cmd Command) error
}

type ValidationCommand struct {
	inner     Command
	validator CommandValidator
	mu        sync.RWMutex
}

func NewValidationCommand(cmd Command, validator CommandValidator) *ValidationCommand {
	return &ValidationCommand{
		inner:     cmd,
		validator: validator,
	}
}

func (vc *ValidationCommand) Execute() error {
	vc.mu.RLock()
	defer vc.mu.RUnlock()

	if err := vc.validator.Validate(vc.inner); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return vc.inner.Execute()
}

func (vc *ValidationCommand) Undo() error {
	vc.mu.RLock()
	defer vc.mu.RUnlock()
	return vc.inner.Undo()
}

func (vc *ValidationCommand) Name() string { return vc.inner.Name() }

type BatchCommand struct {
	commands []Command
	mu       sync.RWMutex
}

func NewBatchCommand(commands ...Command) *BatchCommand {
	return &BatchCommand{commands: commands}
}

func (bc *BatchCommand) Execute() error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for _, cmd := range bc.commands {
		if err := cmd.Execute(); err != nil {
			return fmt.Errorf("batch command %s failed: %w", cmd.Name(), err)
		}
	}
	return nil
}

func (bc *BatchCommand) Undo() error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for i := len(bc.commands) - 1; i >= 0; i-- {
		if err := bc.commands[i].Undo(); err != nil {
			return fmt.Errorf("batch undo %s failed: %w", bc.commands[i].Name(), err)
		}
	}
	return nil
}

func (bc *BatchCommand) Name() string { return "batch" }

type RetryCommand struct {
	inner     Command
	maxRetry  int
	delay     time.Duration
	lastError error
	mu        sync.RWMutex
}

func NewRetryCommand(cmd Command, maxRetry int, delay time.Duration) *RetryCommand {
	if maxRetry <= 0 {
		maxRetry = 3
	}
	if delay <= 0 {
		delay = time.Second
	}
	return &RetryCommand{
		inner:    cmd,
		maxRetry: maxRetry,
		delay:    delay,
	}
}

func (rc *RetryCommand) Execute() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	var err error
	for i := 0; i <= rc.maxRetry; i++ {
		err = rc.inner.Execute()
		if err == nil {
			return nil
		}
		rc.lastError = err
		if i < rc.maxRetry {
			time.Sleep(rc.delay)
		}
	}
	return fmt.Errorf("command failed after %d retries: %w", rc.maxRetry, rc.lastError)
}

func (rc *RetryCommand) Undo() error {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.inner.Undo()
}

func (rc *RetryCommand) Name() string { return rc.inner.Name() }

type LoggingCommand struct {
	inner    Command
	logFunc  func(string, error)
	mu       sync.RWMutex
}

func NewLoggingCommand(cmd Command, logFunc func(string, error)) *LoggingCommand {
	if logFunc == nil {
		logFunc = func(msg string, err error) {}
	}
	return &LoggingCommand{
		inner:   cmd,
		logFunc: logFunc,
	}
}

func (lc *LoggingCommand) Execute() error {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	start := time.Now()
	lc.logFunc(fmt.Sprintf("executing %s", lc.inner.Name()), nil)
	err := lc.inner.Execute()
	duration := time.Since(start)

	if err != nil {
		lc.logFunc(fmt.Sprintf("command %s failed after %v", lc.inner.Name(), duration), err)
	} else {
		lc.logFunc(fmt.Sprintf("command %s completed in %v", lc.inner.Name(), duration), nil)
	}

	return err
}

func (lc *LoggingCommand) Undo() error {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	lc.logFunc(fmt.Sprintf("undoing %s", lc.inner.Name()), nil)
	err := lc.inner.Undo()
	if err != nil {
		lc.logFunc(fmt.Sprintf("undo %s failed", lc.inner.Name()), err)
	}
	return err
}

func (lc *LoggingCommand) Name() string { return lc.inner.Name() }
