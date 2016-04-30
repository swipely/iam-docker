package queue_test

import (
	"errors"
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
	. "github.com/swipely/iam-docker/src/queue"
	"sync"
	"time"
)

type jobStruct struct {
	id              string
	allowedAttempts int
	backoff         func(int) time.Duration
	perform         func() error
}

func (job *jobStruct) ID() string {
	return job.id
}

func (job *jobStruct) AllowedAttempts() int {
	return job.allowedAttempts
}

func (job *jobStruct) Backoff(attempt int) time.Duration {
	return job.backoff(attempt)
}

func (job *jobStruct) Perform() error {
	return job.perform()
}

var _ = Describe("pooledJobQueue", func() {
	const (
		queueSize = 4
		poolSize  = 4
	)

	var (
		jobQueue JobQueue
	)

	BeforeEach(func() {
		jobQueue = NewPooledJobQueue(queueSize, poolSize)
	})

	AfterEach(func() {
		if jobQueue.IsRunning() {
			_ = jobQueue.Stop()
		}
	})

	Describe("Enqueuing a job on a running queue", func() {
		const (
			id              = "test-job"
			allowedAttempts = 2
		)

		var (
			job          *jobStruct
			successGroup sync.WaitGroup
			failGroup    sync.WaitGroup
			shouldFail   bool
		)

		BeforeEach(func() {
			job = &jobStruct{
				id:              id,
				allowedAttempts: allowedAttempts,
				backoff: func(attempt int) time.Duration {
					return 0
				},
				perform: func() error {
					if shouldFail {
						failGroup.Done()
						return errors.New("Job failed")
					}
					successGroup.Done()
					return nil
				},
			}
		})

		Context("When the job succeeds", func() {
			BeforeEach(func() {
				shouldFail = false
			})

			It("Runs the job", func() {
				channel := make(chan struct{})
				successGroup.Add(2)
				go func() {
					err := jobQueue.Run()
					if err != nil {
						Fail(err.Error())
					}
				}()
				go func() {
					successGroup.Wait()
					channel <- struct{}{}
				}()

				jobQueue.Enqueue(job)
				jobQueue.Enqueue(job)
				timer := time.After(5 * time.Second)

				select {
				case <-channel:
					break
				case <-timer:
					Fail("Test timed out")
				}
			})
		})

		Context("When the job fails repeatedly", func() {
			BeforeEach(func() {
				shouldFail = true
			})

			It("Retries until it runs out of attempts", func() {
				channel := make(chan struct{})
				failGroup.Add(4)
				go func() {
					err := jobQueue.Run()
					if err != nil {
						Fail(err.Error())
					}
				}()
				go func() {
					failGroup.Wait()
					channel <- struct{}{}
				}()

				jobQueue.Enqueue(job)
				jobQueue.Enqueue(job)
				timer := time.After(5 * time.Second)

				select {
				case <-channel:
					break
				case <-timer:
					Fail("Test timed out")
				}
			})
		})

		Context("When the job fails and then succeeds", func() {
			BeforeEach(func() {
				job = &jobStruct{
					id:              id,
					allowedAttempts: 10000,
					backoff: func(attempt int) time.Duration {
						return 0
					},
					perform: func() error {
						if shouldFail {
							shouldFail = false
							failGroup.Done()
							return errors.New("Job failed")
						}
						successGroup.Done()
						return nil
					},
				}
			})

			It("Only runs the job until it suceeds", func() {
				channel := make(chan struct{})
				failGroup.Add(1)
				successGroup.Add(1)
				go func() {
					err := jobQueue.Run()
					if err != nil {
						Fail(err.Error())
					}
				}()
				go func() {
					successGroup.Wait()
					failGroup.Wait()
					channel <- struct{}{}
				}()

				jobQueue.Enqueue(job)
				timer := time.After(5 * time.Second)

				select {
				case <-channel:
					break
				case <-timer:
					Fail("Test timed out")
				}
			})
		})
	})
})
