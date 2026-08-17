package pattern

import (
	"context"
	"fmt"
	"time"
)

// Command pattern encapsulates a request as an object, enabling
// undo/redo, queuing, and logging of operations.

// Command defines the interface for executable operations.
type Command interface {
	Execute(ctx context.Context) error
	Undo(ctx context.Context) error
	Description() string
}

// CommandHistory tracks executed commands for undo/redo support.
type CommandHistory struct {
	undoStack []Command
	redoStack []Command
	maxSize   int
}

func NewCommandHistory(maxSize int) *CommandHistory {
	if maxSize <= 0 {
		maxSize = 100
	}
	return &CommandHistory{maxSize: maxSize}
}

func (h *CommandHistory) Execute(ctx context.Context, cmd Command) error {
	if err := cmd.Execute(ctx); err != nil {
		return err
	}
	h.undoStack = append(h.undoStack, cmd)
	h.redoStack = nil // clear redo stack on new command
	if len(h.undoStack) > h.maxSize {
		h.undoStack = h.undoStack[1:]
	}
	return nil
}

func (h *CommandHistory) Undo(ctx context.Context) error {
	if len(h.undoStack) == 0 {
		return fmt.Errorf("nothing to undo")
	}
	last := h.undoStack[len(h.undoStack)-1]
	if err := last.Undo(ctx); err != nil {
		return err
	}
	h.undoStack = h.undoStack[:len(h.undoStack)-1]
	h.redoStack = append(h.redoStack, last)
	return nil
}

func (h *CommandHistory) Redo(ctx context.Context) error {
	if len(h.redoStack) == 0 {
		return fmt.Errorf("nothing to redo")
	}
	last := h.redoStack[len(h.redoStack)-1]
	if err := last.Execute(ctx); err != nil {
		return err
	}
	h.redoStack = h.redoStack[:len(h.redoStack)-1]
	h.undoStack = append(h.undoStack, last)
	return nil
}

func (h *CommandHistory) UndoCount() int { return len(h.undoStack) }
func (h *CommandHistory) RedoCount() int { return len(h.redoStack) }

// RelayCommand is a concrete command for relay operations.
type RelayCommand struct {
	action  func(ctx context.Context) error
	undo    func(ctx context.Context) error
	desc    string
	executed bool
}

func NewRelayCommand(desc string, action, undo func(ctx context.Context) error) *RelayCommand {
	return &RelayCommand{action: action, undo: undo, desc: desc}
}

func (c *RelayCommand) Execute(ctx context.Context) error {
	if err := c.action(ctx); err != nil {
		return err
	}
	c.executed = true
	return nil
}

func (c *RelayCommand) Undo(ctx context.Context) error {
	if !c.executed {
		return fmt.Errorf("command not executed")
	}
	if c.undo != nil {
		return c.undo(ctx)
	}
	return nil
}

func (c *RelayCommand) Description() string { return c.desc }

// DelayedCommand schedules a command for future execution.
type DelayedCommand struct {
	Command
	Delay     time.Duration
	ExecAfter time.Time
}

func NewDelayedCommand(cmd Command, delay time.Duration) *DelayedCommand {
	return &DelayedCommand{
		Command:   cmd,
		Delay:     delay,
		ExecAfter: time.Now().Add(delay),
	}
}

func (d *DelayedCommand) IsReady() bool {
	return time.Now().After(d.ExecAfter)
}

// CommandQueue processes commands in FIFO order.
type CommandQueue struct {
	commands chan Command
	quit     chan struct{}
}

func NewCommandQueue(bufferSize int) *CommandQueue {
	return &CommandQueue{
		commands: make(chan Command, bufferSize),
		quit:     make(chan struct{}),
	}
}

func (q *CommandQueue) Enqueue(cmd Command) {
	q.commands <- cmd
}

func (q *CommandQueue) Start(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		go q.worker(ctx)
	}
}

func (q *CommandQueue) worker(ctx context.Context) {
	for {
		select {
		case <-q.quit:
			return
		case cmd := <-q.commands:
			_ = cmd.Execute(ctx)
		}
	}
}

func (q *CommandQueue) Stop() {
	close(q.quit)
}
