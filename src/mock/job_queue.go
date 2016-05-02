package mock

import (
	"github.com/swipely/iam-docker/src/queue"
)

// NewJobQueue initializes a new mock job queue.
func NewJobQueue() *JobQueue {
	return &JobQueue{
		Jobs: make([]queue.Job, 0),
	}
}

// Enqueue puts a job onto the job queue.
func (queue *JobQueue) Enqueue(job queue.Job) {
	queue.Jobs = append(queue.Jobs, job)
}

// Run does nothing.
func (queue *JobQueue) Run() error {
	return nil
}

// Stop does nothing.
func (queue *JobQueue) Stop() error {
	return nil
}

// IsRunning is always false.
func (queue *JobQueue) IsRunning() bool {
	return false
}

// JobQueue implements queue.JobQueue, but doesn't actually run jobs.
type JobQueue struct {
	Jobs []queue.Job
}
