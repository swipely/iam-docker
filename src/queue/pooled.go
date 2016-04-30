package queue

import (
	"errors"
	"github.com/Sirupsen/logrus"
	"sync"
	"time"
)

// NewPooledJobQueue accepts a queue size and number of workers to create a new
// a new JobQueue which runs jobs asynchronously.
func NewPooledJobQueue(queueSize int, poolSize int) JobQueue {
	jobChan := make(chan *payload, queueSize)
	workerChan := make(chan chan *payload, poolSize)
	workers := make([]*worker, poolSize)
	for i := 0; i < poolSize; i++ {
		workers[i] = newWorker(i+1, jobChan, workerChan)
	}
	return &pooledJobQueue{
		running:    false,
		jobChan:    jobChan,
		workerChan: workerChan,
		workers:    workers,
		stopChan:   make(chan struct{}),
		logger:     log.WithField("job-queue", "async"),
	}
}

func (queue *pooledJobQueue) Enqueue(job Job) {
	queue.logger.WithField("id", job.ID()).Debug("Enqueuing job")
	queue.jobChan <- &payload{
		attempts: 0,
		job:      job,
	}
}

func (queue *pooledJobQueue) Run() error {
	queue.mutex.Lock()
	if queue.running {
		queue.mutex.Unlock()
		return errors.New("Pooled queue is already running")
	}
	queue.running = true
	queue.mutex.Unlock()

	count := len(queue.workers)
	queue.logger.WithField("pool-size", count).Info("Starting workers")

	for _, worker := range queue.workers {
		go worker.work()
	}

	for {
		select {
		case <-queue.stopChan:
			return nil
		case job := <-queue.jobChan:
			wChan := <-queue.workerChan
			wChan <- job
		}
	}
}

func (queue *pooledJobQueue) Stop() error {
	var waitGroup sync.WaitGroup

	queue.mutex.Lock()
	defer queue.mutex.Unlock()

	if !queue.running {
		return errors.New("Pooled queue is not running")
	}
	queue.running = false

	count := len(queue.workers)
	queue.logger.WithField("pool-size", count).Warn("Stopping queue")

	queue.stopChan <- struct{}{}

	waitGroup.Add(count)
	for _, w := range queue.workers {
		go func(worker *worker) {
			worker.stopChan <- struct{}{}
			waitGroup.Done()
		}(w)
	}
	waitGroup.Wait()

	return nil
}

func (queue *pooledJobQueue) IsRunning() bool {
	queue.mutex.Lock()
	defer queue.mutex.Unlock()
	return queue.running
}

func newWorker(id int, upstream chan *payload, workerChan chan chan *payload) *worker {
	return &worker{
		id:         id,
		jobChan:    make(chan *payload),
		upstream:   upstream,
		workerChan: workerChan,
		stopChan:   make(chan struct{}),
		logger:     log.WithField("worker-id", id),
	}
}

func (worker *worker) work() {
	worker.logger.Info("Starting")
	for {
		worker.workerChan <- worker.jobChan

		select {
		case payload := <-worker.jobChan:
			worker.perform(payload)
		case <-worker.stopChan:
			worker.logger.Warn("Stopping worker")
			return
		}
	}
}

func (worker *worker) perform(payload *payload) {
	job := payload.job
	totalAttempts := job.AllowedAttempts()
	logger := worker.logger.WithField("job-id", job.ID())
	logger.Debug("Performing job")
	payload.attempts++
	err := job.Perform()
	if err == nil {
		logger.Debug("Job succeeded")
	} else if totalAttempts > payload.attempts {
		backoff := job.Backoff(payload.attempts)
		logger.WithFields(logrus.Fields{
			"error":              err.Error(),
			"remaining-attempts": totalAttempts - payload.attempts,
			"backoff":            backoff,
		}).Warn("Job failed, retrying")
		go func() {
			time.Sleep(backoff)
			worker.upstream <- payload
		}()
	} else {
		logger.WithField("error", err.Error()).Error("Job failed and is out of retries")
	}
}

type pooledJobQueue struct {
	workers    []*worker
	running    bool
	mutex      sync.Mutex
	jobChan    chan *payload
	workerChan chan chan *payload
	stopChan   chan struct{}
	logger     *logrus.Entry
}

type worker struct {
	id         int
	jobChan    chan *payload
	workerChan chan chan *payload
	upstream   chan *payload
	stopChan   chan struct{}
	logger     *logrus.Entry
}

type payload struct {
	attempts int
	job      Job
}
