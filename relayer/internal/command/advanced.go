package command

import (
	"fmt"
	"sync"
	"time"
)

type TransactionalCommand struct {
	commands  []Command
	executed  []Command
	mu        sync.RWMutex
}

func NewTransactionalCommand(commands ...Command) *TransactionalCommand {
	return &TransactionalCommand{
		commands: commands,
		executed: make([]Command, 0),
	}
}

func (tc *TransactionalCommand) Execute() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.executed = tc.executed[:0]

	for _, cmd := range tc.commands {
		if err := cmd.Execute(); err != nil {
			for i := len(tc.executed) - 1; i >= 0; i-- {
				_ = tc.executed[i].Undo()
			}
			return fmt.Errorf("transaction failed at %s: %w", cmd.Name(), err)
		}
		tc.executed = append(tc.executed, cmd)
	}
	return nil
}

func (tc *TransactionalCommand) Undo() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	var lastErr error
	for i := len(tc.executed) - 1; i >= 0; i-- {
		if err := tc.executed[i].Undo(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func (tc *TransactionalCommand) Name() string { return "transactional" }

type TimeoutCommand struct {
	inner   Command
	timeout time.Duration
	mu      sync.RWMutex
}

func NewTimeoutCommand(cmd Command, timeout time.Duration) *TimeoutCommand {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &TimeoutCommand{inner: cmd, timeout: timeout}
}

func (toc *TimeoutCommand) Execute() error {
	type result struct {
		err error
	}

	ch := make(chan result, 1)
	go func() {
		ch <- result{toc.inner.Execute()}
	}()

	select {
	case r := <-ch:
		return r.err
	case <-time.After(toc.timeout):
		return fmt.Errorf("command %s timed out after %v", toc.inner.Name(), toc.timeout)
	}
}

func (toc *TimeoutCommand) Undo() error {
	return toc.inner.Undo()
}

func (toc *TimeoutCommand) Name() string { return toc.inner.Name() }

type ConditionalCommand struct {
	condition func() bool
	trueCmd   Command
	falseCmd  Command
	mu        sync.RWMutex
}

func NewConditionalCommand(condition func() bool, trueCmd, falseCmd Command) *ConditionalCommand {
	return &ConditionalCommand{
		condition: condition,
		trueCmd:   trueCmd,
		falseCmd:  falseCmd,
	}
}

func (cc *ConditionalCommand) Execute() error {
	cc.mu.RLock()
	cond := cc.condition
	cc.mu.RUnlock()

	if cond() {
		return cc.trueCmd.Execute()
	}
	if cc.falseCmd != nil {
		return cc.falseCmd.Execute()
	}
	return nil
}

func (cc *ConditionalCommand) Undo() error {
	cc.mu.RLock()
	cond := cc.condition
	cc.mu.RUnlock()

	if cond() {
		return cc.trueCmd.Undo()
	}
	if cc.falseCmd != nil {
		return cc.falseCmd.Undo()
	}
	return nil
}

func (cc *ConditionalCommand) Name() string { return "conditional" }

type MementoCommand struct {
	inner    Command
	snapshot interface{}
	save     func() interface{}
	restore  func(interface{})
	mu       sync.RWMutex
}

func NewMementoCommand(cmd Command, save func() interface{}, restore func(interface{})) *MementoCommand {
	return &MementoCommand{
		inner:   cmd,
		save:    save,
		restore: restore,
	}
}

func (mc *MementoCommand) Execute() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.save != nil {
		mc.snapshot = mc.save()
	}
	return mc.inner.Execute()
}

func (mc *MementoCommand) Undo() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.restore != nil && mc.snapshot != nil {
		mc.restore(mc.snapshot)
	}
	return mc.inner.Undo()
}

func (mc *MementoCommand) Name() string { return mc.inner.Name() }

type PriorityCommand struct {
	inner    Command
	priority int
	createdAt time.Time
	mu       sync.RWMutex
}

func NewPriorityCommand(cmd Command, priority int) *PriorityCommand {
	return &PriorityCommand{
		inner:     cmd,
		priority:  priority,
		createdAt: time.Now(),
	}
}

func (pc *PriorityCommand) Execute() error {
	return pc.inner.Execute()
}

func (pc *PriorityCommand) Undo() error {
	return pc.inner.Undo()
}

func (pc *PriorityCommand) Name() string     { return pc.inner.Name() }
func (pc *PriorityCommand) Priority() int    { pc.mu.RLock(); defer pc.mu.RUnlock(); return pc.priority }

type IdempotentCommand struct {
	inner     Command
	id        string
	executed  bool
	mu        sync.RWMutex
}

func NewIdempotentCommand(cmd Command, id string) *IdempotentCommand {
	return &IdempotentCommand{
		inner: cmd,
		id:    id,
	}
}

func (ic *IdempotentCommand) Execute() error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if ic.executed {
		return nil
	}

	if err := ic.inner.Execute(); err != nil {
		return err
	}
	ic.executed = true
	return nil
}

func (ic *IdempotentCommand) Undo() error {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	if !ic.executed {
		return nil
	}

	if err := ic.inner.Undo(); err != nil {
		return err
	}
	ic.executed = false
	return nil
}

func (ic *IdempotentCommand) Name() string { return ic.inner.Name() }

type HistoryAwareCommandInvoker struct {
	history    []Command
	maxHistory int
	mu         sync.RWMutex
}

func NewHistoryAwareCommandInvoker(maxHistory int) *HistoryAwareCommandInvoker {
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &HistoryAwareCommandInvoker{
		history:    make([]Command, 0),
		maxHistory: maxHistory,
	}
}

func (haci *HistoryAwareCommandInvoker) Execute(cmd Command) error {
	haci.mu.Lock()
	defer haci.mu.Unlock()

	if err := cmd.Execute(); err != nil {
		return err
	}

	haci.history = append(haci.history, cmd)
	if len(haci.history) > haci.maxHistory {
		haci.history = haci.history[1:]
	}
	return nil
}

func (haci *HistoryAwareCommandInvoker) Undo() error {
	haci.mu.Lock()
	defer haci.mu.Unlock()

	if len(haci.history) == 0 {
		return fmt.Errorf("no commands to undo")
	}

	last := haci.history[len(haci.history)-1]
	haci.history = haci.history[:len(haci.history)-1]
	return last.Undo()
}

func (haci *HistoryAwareCommandInvoker) History() []Command {
	haci.mu.RLock()
	defer haci.mu.RUnlock()
	result := make([]Command, len(haci.history))
	copy(result, haci.history)
	return result
}

func (haci *HistoryAwareCommandInvoker) Clear() {
	haci.mu.Lock()
	defer haci.mu.Unlock()
	haci.history = haci.history[:0]
}

type SnapshotCommandInvoker struct {
	snapshots map[string][]Command
	mu        sync.RWMutex
}

func NewSnapshotCommandInvoker() *SnapshotCommandInvoker {
	return &SnapshotCommandInvoker{
		snapshots: make(map[string][]Command),
	}
}

func (sci *SnapshotCommandInvoker) Snapshot(name string, history []Command) {
	sci.mu.Lock()
	defer sci.mu.Unlock()
	snap := make([]Command, len(history))
	copy(snap, history)
	sci.snapshots[name] = snap
}

func (sci *SnapshotCommandInvoker) Restore(name string) ([]Command, error) {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	snap, ok := sci.snapshots[name]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", name)
	}
	result := make([]Command, len(snap))
	copy(result, snap)
	return result, nil
}

func (sci *SnapshotCommandInvoker) List() []string {
	sci.mu.RLock()
	defer sci.mu.RUnlock()
	names := make([]string, 0, len(sci.snapshots))
	for name := range sci.snapshots {
		names = append(names, name)
	}
	return names
}

type CommandPriorityQueue struct {
	commands []*PriorityCommand
	mu       sync.RWMutex
}

func NewCommandPriorityQueue() *CommandPriorityQueue {
	return &CommandPriorityQueue{
		commands: make([]*PriorityCommand, 0),
	}
}

func (cpq *CommandPriorityQueue) Push(cmd *PriorityCommand) {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()

	cpq.commands = append(cpq.commands, cmd)
	for i := len(cpq.commands) - 1; i > 0; i-- {
		if cpq.commands[i].priority > cpq.commands[i-1].priority {
			cpq.commands[i], cpq.commands[i-1] = cpq.commands[i-1], cpq.commands[i]
		} else {
			break
		}
	}
}

func (cpq *CommandPriorityQueue) Pop() *PriorityCommand {
	cpq.mu.Lock()
	defer cpq.mu.Unlock()

	if len(cpq.commands) == 0 {
		return nil
	}
	cmd := cpq.commands[0]
	cpq.commands = cpq.commands[1:]
	return cmd
}

func (cpq *CommandPriorityQueue) Size() int {
	cpq.mu.RLock()
	defer cpq.mu.RUnlock()
	return len(cpq.commands)
}

func (cpq *CommandPriorityQueue) Peek() *PriorityCommand {
	cpq.mu.RLock()
	defer cpq.mu.RUnlock()
	if len(cpq.commands) == 0 {
		return nil
	}
	return cpq.commands[0]
}
