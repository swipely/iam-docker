package queue

import (
	"github.com/Sirupsen/logrus"
	"time"
)

var (
	log = logrus.WithField("prefix", "queue")
)

// JobQueue performs and retries jobs.
type JobQueue interface {
	// Enqueue a job to be performed.
	Enqueue(Job)
	// Run the job queue synchronously.
	Run() error
	// Stop the Queue and wait for all running jobs to complete or fail.
	Stop() error
	// Determine of the queue is running.
	IsRunning() bool
}

// Job represents a unit of work that can be run by a worker.
type Job interface {
	// Job identifier.
	ID() string
	// Maximum number of times the job may be attempted when a failure
	// occurs.
	AllowedAttempts() int
	// Given the attempt number, this function returns how long the job
	// should sleep before retrying on failure.
	Backoff(int) time.Duration
	// Function which actually runs the job.
	Perform() error
}
