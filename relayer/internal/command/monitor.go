package command

import (
	"sync"
	"time"
)

type CommandMonitor struct {
	executed   int64
	failed     int64
	undone     int64
	totalTime  time.Duration
	maxTime    time.Duration
	minTime    time.Duration
	lastExec   time.Time
	mu         sync.RWMutex
}

func NewCommandMonitor() *CommandMonitor {
	return &CommandMonitor{
		minTime: time.Duration(1<<63 - 1),
	}
}

func (cm *CommandMonitor) RecordExecution(duration time.Duration, success bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.executed++
	cm.totalTime += duration
	cm.lastExec = time.Now()

	if duration > cm.maxTime {
		cm.maxTime = duration
	}
	if duration < cm.minTime {
		cm.minTime = duration
	}

	if !success {
		cm.failed++
	}
}

func (cm *CommandMonitor) RecordUndo(duration time.Duration, success bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !success {
		cm.undone++
	}
}

func (cm *CommandMonitor) Stats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var avgTime time.Duration
	if cm.executed > 0 {
		avgTime = cm.totalTime / time.Duration(cm.executed)
	}

	return map[string]interface{}{
		"executed":  cm.executed,
		"failed":    cm.failed,
		"undone":    cm.undone,
		"total_time": cm.totalTime,
		"avg_time":  avgTime,
		"max_time":  cm.maxTime,
		"min_time":  cm.minTime,
		"last_exec": cm.lastExec,
	}
}

func (cm *CommandMonitor) Reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.executed = 0
	cm.failed = 0
	cm.undone = 0
	cm.totalTime = 0
	cm.maxTime = 0
	cm.minTime = time.Duration(1<<63 - 1)
}

type CommandLogger struct {
	entries []CommandLogEntry
	maxSize int
	mu      sync.RWMutex
}

type CommandLogEntry struct {
	Command   string
	Action    string
	Duration  time.Duration
	Success   bool
	Error     string
	Timestamp time.Time
}

func NewCommandLogger(maxSize int) *CommandLogger {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &CommandLogger{
		entries: make([]CommandLogEntry, 0),
		maxSize: maxSize,
	}
}

func (cl *CommandLogger) Log(entry CommandLogEntry) {
	cl.mu.Lock()
	defer cl.mu.Unlock()

	if len(cl.entries) >= cl.maxSize {
		cl.entries = cl.entries[1:]
	}
	entry.Timestamp = time.Now()
	cl.entries = append(cl.entries, entry)
}

func (cl *CommandLogger) GetRecent(count int) []CommandLogEntry {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	if count > len(cl.entries) {
		count = len(cl.entries)
	}
	result := make([]CommandLogEntry, count)
	copy(result, cl.entries[len(cl.entries)-count:])
	return result
}

func (cl *CommandLogger) SearchByName(name string) []CommandLogEntry {
	cl.mu.RLock()
	defer cl.mu.RUnlock()

	var results []CommandLogEntry
	for _, entry := range cl.entries {
		if entry.Command == name {
			results = append(results, entry)
		}
	}
	return results
}

func (cl *CommandLogger) Clear() {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	cl.entries = cl.entries[:0]
}

type CommandThrottler struct {
	limit    int
	window   time.Duration
	counts   map[string]int
	lastTime map[string]time.Time
	mu       sync.RWMutex
}

func NewCommandThrottler(limit int, window time.Duration) *CommandThrottler {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return &CommandThrottler{
		limit:    limit,
		window:   window,
		counts:   make(map[string]int),
		lastTime: make(map[string]time.Time),
	}
}

func (ct *CommandThrottler) Allow(name string) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	now := time.Now()
	if now.Sub(ct.lastTime[name]) > ct.window {
		ct.counts[name] = 0
		ct.lastTime[name] = now
	}

	ct.counts[name]++
	return ct.counts[name] <= ct.limit
}

func (ct *CommandThrottler) Reset(name string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.counts[name] = 0
	ct.lastTime[name] = time.Time{}
}

type CommandDeduplicator struct {
	seen   map[string]time.Time
	ttl    time.Duration
	mu     sync.RWMutex
}

func NewCommandDeduplicator(ttl time.Duration) *CommandDeduplicator {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &CommandDeduplicator{
		seen: make(map[string]time.Time),
		ttl:  ttl,
	}
}

func (cd *CommandDeduplicator) IsDuplicate(id string) bool {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	cd.cleanup()

	if lastSeen, ok := cd.seen[id]; ok {
		if time.Since(lastSeen) < cd.ttl {
			return true
		}
	}

	cd.seen[id] = time.Now()
	return false
}

func (cd *CommandDeduplicator) cleanup() {
	now := time.Now()
	for id, lastSeen := range cd.seen {
		if now.Sub(lastSeen) > cd.ttl {
			delete(cd.seen, id)
		}
	}
}

type CommandChain struct {
	commands []Command
	index    int
	mu       sync.RWMutex
}

func NewCommandChain(commands ...Command) *CommandChain {
	return &CommandChain{
		commands: commands,
		index:    0,
	}
}

func (cc *CommandChain) ExecuteNext() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.index >= len(cc.commands) {
		return nil
	}

	cmd := cc.commands[cc.index]
	cc.index++
	return cmd.Execute()
}

func (cc *CommandChain) ExecuteAll() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	for _, cmd := range cc.commands {
		if err := cmd.Execute(); err != nil {
			return err
		}
	}
	cc.index = len(cc.commands)
	return nil
}

func (cc *CommandChain) UndoAll() error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	var lastErr error
	for i := len(cc.commands) - 1; i >= 0; i-- {
		if err := cc.commands[i].Undo(); err != nil {
			lastErr = err
		}
	}
	cc.index = 0
	return lastErr
}

func (cc *CommandChain) Position() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.index
}

func (cc *CommandChain) Len() int {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return len(cc.commands)
}

type CommandGroup struct {
	commands  map[string]Command
	order     []string
	executed  map[string]bool
	mu        sync.RWMutex
}

func NewCommandGroup() *CommandGroup {
	return &CommandGroup{
		commands: make(map[string]Command),
		order:    make([]string, 0),
		executed: make(map[string]bool),
	}
}

func (cg *CommandGroup) Add(name string, cmd Command) {
	cg.mu.Lock()
	defer cg.mu.Unlock()
	cg.commands[name] = cmd
	cg.order = append(cg.order, name)
}

func (cg *CommandGroup) Execute(name string) error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	cmd, ok := cg.commands[name]
	if !ok {
		return nil
	}

	if err := cmd.Execute(); err != nil {
		return err
	}
	cg.executed[name] = true
	return nil
}

func (cg *CommandGroup) ExecuteAll() error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	for _, name := range cg.order {
		if err := cg.commands[name].Execute(); err != nil {
			return err
		}
		cg.executed[name] = true
	}
	return nil
}

func (cg *CommandGroup) UndoAll() error {
	cg.mu.Lock()
	defer cg.mu.Unlock()

	var lastErr error
	for i := len(cg.order) - 1; i >= 0; i-- {
		name := cg.order[i]
		if cg.executed[name] {
			if err := cg.commands[name].Undo(); err != nil {
				lastErr = err
			}
			cg.executed[name] = false
		}
	}
	return lastErr
}

func (cg *CommandGroup) IsExecuted(name string) bool {
	cg.mu.RLock()
	defer cg.mu.RUnlock()
	return cg.executed[name]
}

type CommandScope struct {
	parent   *CommandScope
	commands []Command
	mu       sync.RWMutex
}

func NewCommandScope(parent *CommandScope) *CommandScope {
	return &CommandScope{
		parent:   parent,
		commands: make([]Command, 0),
	}
}

func (cs *CommandScope) Add(cmd Command) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.commands = append(cs.commands, cmd)
}

func (cs *CommandScope) ExecuteAll() error {
	cs.mu.RLock()
	commands := make([]Command, len(cs.commands))
	copy(commands, cs.commands)
	cs.mu.RUnlock()

	for _, cmd := range commands {
		if err := cmd.Execute(); err != nil {
			return err
		}
	}
	return nil
}

func (cs *CommandScope) ExecuteWithParent() error {
	if cs.parent != nil {
		if err := cs.parent.ExecuteAll(); err != nil {
			return err
		}
	}
	return cs.ExecuteAll()
}

func (cs *CommandScope) UndoAll() error {
	cs.mu.RLock()
	commands := make([]Command, len(cs.commands))
	copy(commands, cs.commands)
	cs.mu.RUnlock()

	var lastErr error
	for i := len(commands) - 1; i >= 0; i-- {
		if err := commands[i].Undo(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

type AsyncCommandExecutor struct {
	results chan CommandResult
	workers int
	mu      sync.RWMutex
}

type CommandResult struct {
	Name    string
	Error   error
	Done    time.Duration
}

func NewAsyncCommandExecutor(workers int) *AsyncCommandExecutor {
	if workers <= 0 {
		workers = 4
	}
	return &AsyncCommandExecutor{
		results: make(chan CommandResult, 100),
		workers: workers,
	}
}

func (ace *AsyncCommandExecutor) ExecuteAsync(cmd Command) {
	go func() {
		start := time.Now()
		err := cmd.Execute()
		ace.results <- CommandResult{
			Name:  cmd.Name(),
			Error: err,
			Done:  time.Since(start),
		}
	}()
}

func (ace *AsyncCommandExecutor) Results() <-chan CommandResult {
	return ace.results
}

type CommandBatch struct {
	commands []Command
	batchSize int
	mu       sync.RWMutex
}

func NewCommandBatch(batchSize int) *CommandBatch {
	if batchSize <= 0 {
		batchSize = 10
	}
	return &CommandBatch{
		commands:  make([]Command, 0),
		batchSize: batchSize,
	}
}

func (cb *CommandBatch) Add(cmd Command) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.commands = append(cb.commands, cmd)
}

func (cb *CommandBatch) ExecuteAll() error {
	cb.mu.RLock()
	commands := make([]Command, len(cb.commands))
	copy(commands, cb.commands)
	cb.mu.RUnlock()

	for i := 0; i < len(commands); i += cb.batchSize {
		end := i + cb.batchSize
		if end > len(commands) {
			end = len(commands)
		}

		for _, cmd := range commands[i:end] {
			if err := cmd.Execute(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cb *CommandBatch) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return len(cb.commands)
}

func (cb *CommandBatch) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.commands = cb.commands[:0]
}
