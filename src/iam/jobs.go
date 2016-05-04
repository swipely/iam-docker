package iam

import (
	"github.com/Sirupsen/logrus"
	"github.com/swipely/iam-docker/src/queue"
	"github.com/swipely/iam-docker/src/utils"
	"time"
)

// refreshCredentialJob refreshes the credentials for a single ARN.
type refreshCredentialJob struct {
	arn             string
	credentialStore CredentialStore
	logger          *logrus.Entry
}

// NewRefreshCredentialJob creates a new refreshCredentialJob.
func NewRefreshCredentialJob(arn string, credentialStore CredentialStore) queue.Job {
	return &refreshCredentialJob{
		arn:             arn,
		credentialStore: credentialStore,
		logger: logrus.WithFields(logrus.Fields{
			"prefix": "iam/refresh-credential",
			"arn":    arn,
		}),
	}
}

func (job *refreshCredentialJob) ID() string {
	return "iam/refresh-credential/" + job.arn
}

func (job *refreshCredentialJob) AllowedAttempts() int {
	return 3
}

func (job *refreshCredentialJob) Backoff(attempt int) time.Duration {
	return utils.ExponentialBackoff(time.Second, attempt)
}

func (job *refreshCredentialJob) Perform() error {
	job.logger.Debug("Refreshing credential")
	err := job.credentialStore.RefreshCredentialIfStale(job.arn)
	if err != nil {
		return err
	}
	return nil
}

// refreshCredentialsJob refreshes the credentials for a each ARN in the
// CredentialStore via multiple background jobs.
type refreshCredentialsJob struct {
	credentialStore CredentialStore
	jobQueue        queue.JobQueue
}

// NewRefreshCredentialsJob creates a new job to refresh all IAM credentials.
func NewRefreshCredentialsJob(credentialStore CredentialStore, jobQueue queue.JobQueue) queue.Job {
	return &refreshCredentialsJob{
		credentialStore: credentialStore,
		jobQueue:        jobQueue,
	}
}

func (job *refreshCredentialsJob) ID() string {
	return "iam/refresh-credentials"
}

func (job *refreshCredentialsJob) AllowedAttempts() int {
	return 1
}

func (job *refreshCredentialsJob) Backoff(attempt int) time.Duration {
	return 0
}

func (job *refreshCredentialsJob) Perform() error {
	arns := job.credentialStore.AvailableARNs()
	for _, arn := range arns {
		refreshJob := NewRefreshCredentialJob(arn, job.credentialStore)
		job.jobQueue.Enqueue(refreshJob)
	}
	return nil
}
