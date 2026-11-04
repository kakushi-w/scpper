package sync

import (
	"sort"
	gosync "sync"
	"time"
)

type Job struct {
	Kind       string     `json:"kind"`
	Target     string     `json:"target"`
	Done       int        `json:"done"`
	Total      int        `json:"total"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	Err        string     `json:"error,omitempty"`
}

type jobBoard struct {
	mu   gosync.Mutex
	jobs map[string]*Job
}

const (
	keepDoneJobs   = time.Minute
	keepFailedJobs = 30 * time.Minute
)

func (b *jobBoard) start(kind, target string, total int) *Job {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.jobs == nil {
		b.jobs = map[string]*Job{}
	}
	j := &Job{Kind: kind, Target: target, Total: total, StartedAt: time.Now()}
	b.jobs[kind+" "+target] = j
	return j
}

func (b *jobBoard) progress(j *Job, done, total int) {
	if j == nil {
		return
	}
	b.mu.Lock()
	j.Done = done
	if total > 0 {
		j.Total = total
	}
	b.mu.Unlock()
}

func (b *jobBoard) finish(j *Job, err error) {
	if j == nil {
		return
	}
	b.mu.Lock()
	now := time.Now()
	j.FinishedAt = &now
	if err != nil {
		j.Err = err.Error()
	}
	b.mu.Unlock()
}

func (b *jobBoard) running(kind, target string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	j, ok := b.jobs[kind+" "+target]
	return ok && j.FinishedAt == nil
}

func (b *jobBoard) list() []Job {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := []Job{}
	for key, j := range b.jobs {
		keep := keepDoneJobs
		if j.Err != "" {
			keep = keepFailedJobs
		}
		if j.FinishedAt != nil && time.Since(*j.FinishedAt) > keep {
			delete(b.jobs, key)
			continue
		}
		out = append(out, *j)
	}
	sort.Slice(out, func(a, c int) bool { return out[a].StartedAt.After(out[c].StartedAt) })
	return out
}

func (e *Engine) Jobs() []Job { return e.jobs.list() }

func (e *Engine) JobRunning(kind, target string) bool { return e.jobs.running(kind, target) }
