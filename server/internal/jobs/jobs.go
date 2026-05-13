package jobs

import (
	"context"
	"log"
	"sync"
	"time"
)

// Job represents a scheduled background task
type Job interface {
	// Name returns the job name for logging
	Name() string

	// Run executes the job
	Run(ctx context.Context) error

	// Interval returns how often the job should run
	Interval() time.Duration
}

// Scheduler manages and runs scheduled jobs
type Scheduler struct {
	jobs   []Job
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex
}

// NewScheduler creates a new job scheduler
func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		jobs:   make([]Job, 0),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Register adds a job to the scheduler
func (s *Scheduler) Register(job Job) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, job)
	log.Printf("[Jobs] Registered: %s (runs every %v)", job.Name(), job.Interval())
}

// Start begins running all registered jobs
func (s *Scheduler) Start() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, job := range s.jobs {
		s.wg.Add(1)
		go s.runJob(job)
	}

	log.Printf("[Jobs] Started %d background jobs", len(s.jobs))
}

// Stop gracefully stops all running jobs
func (s *Scheduler) Stop() {
	log.Println("[Jobs] Stopping scheduler...")
	s.cancel()
	s.wg.Wait()
	log.Println("[Jobs] All jobs stopped")
}

// runJob runs a single job on its interval
func (s *Scheduler) runJob(job Job) {
	defer s.wg.Done()

	ticker := time.NewTicker(job.Interval())
	defer ticker.Stop()

	// Run immediately on start
	if err := job.Run(s.ctx); err != nil {
		log.Printf("[Jobs] %s failed: %v", job.Name(), err)
	} else {
		log.Printf("[Jobs] %s completed successfully", job.Name())
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := job.Run(s.ctx); err != nil {
				log.Printf("[Jobs] %s failed: %v", job.Name(), err)
			} else {
				log.Printf("[Jobs] %s completed successfully", job.Name())
			}
		}
	}
}
