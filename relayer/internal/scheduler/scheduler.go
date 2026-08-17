package scheduler

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Job struct {
	ID         string
	Name       string
	Schedule   string
	Fn         func() error
	Enabled    bool
	MaxRuns    int
	RunCount   int
	LastRun    time.Time
	NextRun    time.Time
	Metadata   map[string]string
	mu         sync.RWMutex
}

func (j *Job) ShouldRun(now time.Time) bool {
	j.mu.RLock()
	defer j.mu.RUnlock()
	if !j.Enabled {
		return false
	}
	if j.MaxRuns > 0 && j.RunCount >= j.MaxRuns {
		return false
	}
	return now.After(j.NextRun) || now.Equal(j.NextRun)
}

func (j *Job) MarkRun() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.RunCount++
	j.LastRun = time.Now()
}

func (j *Job) Disable() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Enabled = false
}

func (j *Job) Enable() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.Enabled = true
}

type Scheduler struct {
	jobs    map[string]*Job
	mu      sync.RWMutex
	stopCh  chan struct{}
	running bool
	onError func(string, error)
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		jobs:   make(map[string]*Job),
		stopCh: make(chan struct{}),
	}
}

func (s *Scheduler) AddJob(job *Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs[job.ID] = job
}

func (s *Scheduler) RemoveJob(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, id)
}

func (s *Scheduler) GetJob(id string) *Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.jobs[id]
}

func (s *Scheduler) UpdateJob(id string, fn func(*Job)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job, exists := s.jobs[id]; exists {
		fn(job)
	}
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.runDueJobs()
			case <-s.stopCh:
				return
			}
		}
	}()
}

func (s *Scheduler) runDueJobs() {
	s.mu.RLock()
	now := time.Now()
	dueJobs := make([]*Job, 0)
	for _, job := range s.jobs {
		if job.ShouldRun(now) {
			dueJobs = append(dueJobs, job)
		}
	}
	s.mu.RUnlock()

	for _, job := range dueJobs {
		go func(j *Job) {
			if err := j.Fn(); err != nil {
				if s.onError != nil {
					s.onError(j.ID, err)
				}
			}
			j.MarkRun()
		}(job)
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
}

func (s *Scheduler) OnError(fn func(string, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onError = fn
}

func (s *Scheduler) JobCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.jobs)
}

func (s *Scheduler) Running() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Scheduler) ListJobs() []*Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].NextRun.Before(jobs[j].NextRun)
	})
	return jobs
}

type CronExpression struct {
	Minutes   []int
	Hours     []int
	Days      []int
	Months    []int
	Weekdays  []int
	Raw       string
	mu        sync.RWMutex
}

func ParseCron(expr string) (*CronExpression, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("invalid cron expression: expected 5 fields, got %d", len(parts))
	}

	ce := &CronExpression{Raw: expr}

	minutes, err := parseField(parts[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("invalid minute field: %w", err)
	}
	ce.Minutes = minutes

	hours, err := parseField(parts[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("invalid hour field: %w", err)
	}
	ce.Hours = hours

	days, err := parseField(parts[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("invalid day field: %w", err)
	}
	ce.Days = days

	months, err := parseField(parts[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("invalid month field: %w", err)
	}
	ce.Months = months

	weekdays, err := parseField(parts[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("invalid weekday field: %w", err)
	}
	ce.Weekdays = weekdays

	return ce, nil
}

func parseField(field string, min, max int) ([]int, error) {
	if field == "*" {
		result := make([]int, 0)
		for i := min; i <= max; i++ {
			result = append(result, i)
		}
		return result, nil
	}

	result := make([]int, 0)
	parts := strings.Split(field, ",")
	for _, part := range parts {
		if strings.Contains(part, "/") {
			slashParts := strings.Split(part, "/")
			if len(slashParts) != 2 {
				return nil, fmt.Errorf("invalid step: %s", part)
			}
			step := 0
			_, err := fmt.Sscanf(slashParts[1], "%d", &step)
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step value: %s", slashParts[1])
			}

			start := min
			if slashParts[0] != "*" {
				_, err := fmt.Sscanf(slashParts[0], "%d", &start)
				if err != nil {
					return nil, fmt.Errorf("invalid range start: %s", slashParts[0])
				}
			}

			for i := start; i <= max; i += step {
				result = append(result, i)
			}
		} else if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, end := 0, 0
			_, err := fmt.Sscanf(rangeParts[0], "%d", &start)
			if err != nil {
				return nil, fmt.Errorf("invalid range start: %s", rangeParts[0])
			}
			_, err = fmt.Sscanf(rangeParts[1], "%d", &end)
			if err != nil {
				return nil, fmt.Errorf("invalid range end: %s", rangeParts[1])
			}
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			val := 0
			_, err := fmt.Sscanf(part, "%d", &val)
			if err != nil {
				return nil, fmt.Errorf("invalid value: %s", part)
			}
			if val < min || val > max {
				return nil, fmt.Errorf("value out of range: %d", val)
			}
			result = append(result, val)
		}
	}

	return result, nil
}

func (ce *CronExpression) Matches(t time.Time) bool {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	if !contains(ce.Minutes, t.Minute()) {
		return false
	}
	if !contains(ce.Hours, t.Hour()) {
		return false
	}
	if !contains(ce.Days, t.Day()) {
		return false
	}
	if !contains(ce.Months, int(t.Month())) {
		return false
	}
	if !contains(ce.Weekdays, int(t.Weekday())) {
		return false
	}
	return true
}

func (ce *CronExpression) NextTime(from time.Time) time.Time {
	next := from.Add(time.Minute)
	for i := 0; i < 366*24*60; i++ {
		if ce.Matches(next) {
			return next
		}
		next = next.Add(time.Minute)
	}
	return time.Time{}
}

func contains(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

type CronScheduler struct {
	scheduler *Scheduler
	mu        sync.RWMutex
}

func NewCronScheduler() *CronScheduler {
	return &CronScheduler{
		scheduler: NewScheduler(),
	}
}

func (cs *CronScheduler) AddCronJob(name, cronExpr string, fn func() error) (*Job, error) {
	expr, err := ParseCron(cronExpr)
	if err != nil {
		return nil, err
	}

	job := &Job{
		ID:       fmt.Sprintf("cron_%d", time.Now().UnixNano()),
		Name:     name,
		Schedule: cronExpr,
		Fn:       fn,
		Enabled:  true,
		NextRun:  expr.NextTime(time.Now()),
		Metadata: make(map[string]string),
	}

	cs.scheduler.AddJob(job)
	return job, nil
}

func (cs *CronScheduler) Start() {
	cs.scheduler.Start()
}

func (cs *CronScheduler) Stop() {
	cs.scheduler.Stop()
}

type JobStore struct {
	jobs  map[string]*Job
	mu    sync.RWMutex
}

func NewJobStore() *JobStore {
	return &JobStore{
		jobs: make(map[string]*Job),
	}
}

func (js *JobStore) Save(job *Job) {
	js.mu.Lock()
	defer js.mu.Unlock()
	js.jobs[job.ID] = job
}

func (js *JobStore) Get(id string) *Job {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.jobs[id]
}

func (js *JobStore) Delete(id string) {
	js.mu.Lock()
	defer js.mu.Unlock()
	delete(js.jobs, id)
}

func (js *JobStore) List() []*Job {
	js.mu.RLock()
	defer js.mu.RUnlock()
	jobs := make([]*Job, 0, len(js.jobs))
	for _, job := range js.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func (js *JobStore) Count() int {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return len(js.jobs)
}

type DelayedJob struct {
	Job     *Job
	Delay   time.Duration
	Created time.Time
	RunAt   time.Time
}

type DelayedJobScheduler struct {
	jobs   []*DelayedJob
	mu     sync.RWMutex
	stopCh chan struct{}
}

func NewDelayedJobScheduler() *DelayedJobScheduler {
	return &DelayedJobScheduler{
		jobs:   make([]*DelayedJob, 0),
		stopCh: make(chan struct{}),
	}
}

func (djs *DelayedJobScheduler) AddJob(job *Job, delay time.Duration) {
	djs.mu.Lock()
	defer djs.mu.Unlock()

	now := time.Now()
	djs.jobs = append(djs.jobs, &DelayedJob{
		Job:     job,
		Delay:   delay,
		Created: now,
		RunAt:   now.Add(delay),
	})
}

func (djs *DelayedJobScheduler) Start() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				djs.runDueJobs()
			case <-djs.stopCh:
				return
			}
		}
	}()
}

func (djs *DelayedJobScheduler) runDueJobs() {
	djs.mu.Lock()
	now := time.Now()
	due := make([]*DelayedJob, 0)
	remaining := make([]*DelayedJob, 0)

	for _, dj := range djs.jobs {
		if now.After(dj.RunAt) || now.Equal(dj.RunAt) {
			due = append(due, dj)
		} else {
			remaining = append(remaining, dj)
		}
	}
	djs.jobs = remaining
	djs.mu.Unlock()

	for _, dj := range due {
		go func(job *Job) {
			job.Fn()
		}(dj.Job)
	}
}

func (djs *DelayedJobScheduler) Stop() {
	close(djs.stopCh)
}

func (djs *DelayedJobScheduler) PendingCount() int {
	djs.mu.RLock()
	defer djs.mu.RUnlock()
	return len(djs.jobs)
}

type JobLock struct {
	locks map[string]bool
	mu    sync.Mutex
}

func NewJobLock() *JobLock {
	return &JobLock{
		locks: make(map[string]bool),
	}
}

func (jl *JobLock) TryLock(jobID string) bool {
	jl.mu.Lock()
	defer jl.mu.Unlock()

	if jl.locks[jobID] {
		return false
	}
	jl.locks[jobID] = true
	return true
}

func (jl *JobLock) Unlock(jobID string) {
	jl.mu.Lock()
	defer jl.mu.Unlock()
	delete(jl.locks, jobID)
}

func (jl *JobLock) IsLocked(jobID string) bool {
	jl.mu.Lock()
	defer jl.mu.Unlock()
	return jl.locks[jobID]
}

type DistributedJobLock struct {
	locks   map[string]lockEntry
	keyFunc func(string) string
	mu      sync.Mutex
}

type lockEntry struct {
	owner   string
	expires time.Time
}

func NewDistributedJobLock() *DistributedJobLock {
	return &DistributedJobLock{
		locks: make(map[string]lockEntry),
	}
}

func (djl *DistributedJobLock) TryLock(jobID, owner string, ttl time.Duration) bool {
	djl.mu.Lock()
	defer djl.mu.Unlock()

	if entry, exists := djl.locks[jobID]; exists {
		if time.Now().Before(entry.expires) {
			return false
		}
	}

	djl.locks[jobID] = lockEntry{
		owner:   owner,
		expires: time.Now().Add(ttl),
	}
	return true
}

func (djl *DistributedJobLock) Unlock(jobID, owner string) bool {
	djl.mu.Lock()
	defer djl.mu.Unlock()

	entry, exists := djl.locks[jobID]
	if !exists || entry.owner != owner {
		return false
	}

	delete(djl.locks, jobID)
	return true
}

func (djl *DistributedJobLock) Cleanup() {
	djl.mu.Lock()
	defer djl.mu.Unlock()

	now := time.Now()
	for id, entry := range djl.locks {
		if now.After(entry.expires) {
			delete(djl.locks, id)
		}
	}
}

type JobMetrics struct {
	TotalRuns   int64
	SuccessRuns int64
	FailedRuns  int64
	TotalTime   time.Duration
	AverageTime time.Duration
	mu          sync.RWMutex
}

func NewJobMetrics() *JobMetrics {
	return &JobMetrics{}
}

func (jm *JobMetrics) RecordRun(duration time.Duration, success bool) {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.TotalRuns++
	jm.TotalTime += duration
	jm.AverageTime = jm.TotalTime / time.Duration(jm.TotalRuns)
	if success {
		jm.SuccessRuns++
	} else {
		jm.FailedRuns++
	}
}

func (jm *JobMetrics) Snapshot() JobMetricsSnapshot {
	jm.mu.RLock()
	defer jm.mu.RUnlock()
	return JobMetricsSnapshot{
		TotalRuns:   jm.TotalRuns,
		SuccessRuns: jm.SuccessRuns,
		FailedRuns:  jm.FailedRuns,
		TotalTime:   jm.TotalTime,
		AverageTime: jm.AverageTime,
	}
}

func (jm *JobMetrics) Reset() {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	jm.TotalRuns = 0
	jm.SuccessRuns = 0
	jm.FailedRuns = 0
	jm.TotalTime = 0
	jm.AverageTime = 0
}

type JobMetricsSnapshot struct {
	TotalRuns   int64
	SuccessRuns int64
	FailedRuns  int64
	TotalTime   time.Duration
	AverageTime time.Duration
}
