package docker

import (
	"github.com/Sirupsen/logrus"
	"github.com/swipely/iam-docker/src/iam"
	"github.com/swipely/iam-docker/src/queue"
	"github.com/swipely/iam-docker/src/utils"
	"time"
)

// addContainerJob is a background job which adds a container to its
// ContainerStore and fetches the credential for its CredentialStore.
type addContainerJob struct {
	containerID     string
	containerStore  ContainerStore
	credentialStore iam.CredentialStore
	logger          *logrus.Entry
}

// NewAddContainerJob initializes an addContainerJob.
func NewAddContainerJob(containerID string, containerStore ContainerStore, credentialStore iam.CredentialStore) queue.Job {
	return &addContainerJob{
		containerID:     containerID,
		containerStore:  containerStore,
		credentialStore: credentialStore,
		logger: logrus.WithFields(logrus.Fields{
			"prefix":       "docker/add-container",
			"container-id": containerID,
		}),
	}
}

// ID retruns a unique identifier for an addContainerJob.
func (job *addContainerJob) ID() string {
	return "docker/add-container/" + job.containerID
}

// AllowedAttempts is the maximum number of times an addContainerJob may be run.
func (job *addContainerJob) AllowedAttempts() int {
	return 3
}

// Backoff returns the duration which should be given between attempts.
func (job *addContainerJob) Backoff(attempt int) time.Duration {
	return utils.ExponentialBackoff(time.Second, attempt)
}

// Perform attempts to add a container to the job's ContainerStore and assume
// its role with the credentialStore.
func (job *addContainerJob) Perform() error {
	job.logger.Info("Adding container")
	err := job.containerStore.AddContainerByID(job.containerID)
	if err != nil {
		return err
	}
	role, err := job.containerStore.IAMRoleForID(job.containerID)
	if err != nil {
		return err
	}
	job.logger.WithField("iam-role", role).Debug("Fetching IAM Role")
	err = job.credentialStore.RefreshCredentialIfStale(role)
	if err != nil {
		return err
	}
	return nil
}

// removeContainerJob is a background job which removes the container from its
// containerStore.
type removeContainerJob struct {
	containerID    string
	containerStore ContainerStore
	logger         *logrus.Entry
}

// NewRemoveContainerJob initializes a removeContainerJob.
func NewRemoveContainerJob(containerID string, containerStore ContainerStore) queue.Job {
	return &removeContainerJob{
		containerID:    containerID,
		containerStore: containerStore,
		logger: logrus.WithFields(logrus.Fields{
			"prefix":       "docker/remove-container",
			"container-id": containerID,
		}),
	}
}

// ID retruns a unique identifier for an removeContainerJob.
func (job *removeContainerJob) ID() string {
	return "docker/remove-container/" + job.containerID
}

// AllowedAttempts is the maximum number of times an removeContainerJob may be run.
func (job *removeContainerJob) AllowedAttempts() int {
	return 1
}

// Backoff returns the duration which should be given between attempts.
func (job *removeContainerJob) Backoff(attempt int) time.Duration {
	return 0
}

// Perform removes the container from the job's ContainerStore.
func (job *removeContainerJob) Perform() error {
	job.logger.Info("Removing container")
	job.containerStore.RemoveContainer(job.containerID)
	return nil
}

// syncContainersJob is a background job which synchronizes the running
// containers in Docker to its ContainerStore. It enqueues addContainerJobs
// for each container it finds.
type syncContainersJob struct {
	client          RawClient
	containerStore  ContainerStore
	credentialStore iam.CredentialStore
	queue           queue.JobQueue
	logger          *logrus.Entry
}

// NewSyncContainersJob creates a syncContainersJob.
func NewSyncContainersJob(client RawClient, containerStore ContainerStore, credentialStore iam.CredentialStore, queue queue.JobQueue) queue.Job {
	return &syncContainersJob{
		client:          client,
		containerStore:  containerStore,
		credentialStore: credentialStore,
		queue:           queue,
		logger: logrus.WithFields(logrus.Fields{
			"prefix": "docker/sync-containers",
		}),
	}
}

// ID retruns a unique identifier for an syncContainersJob.
func (job *syncContainersJob) ID() string {
	return "docker/sync-containers"
}

// AllowedAttempts is the maximum number of times an syncContainersJob may be run.
func (job *syncContainersJob) AllowedAttempts() int {
	return 3
}

// Backoff returns the duration which should be given between attempts.
func (job *syncContainersJob) Backoff(attempt int) time.Duration {
	return utils.ExponentialBackoff(time.Second, attempt)
}

// Perform attempts to add a container to the job's ContainerStore and
func (job *syncContainersJob) Perform() error {
	job.logger.Info("Syncing running containers")
	containers, err := job.client.ListContainers(runningContainersOpts)
	if err != nil {
		return err
	}
	job.logger.WithField(
		"containers",
		len(containers),
	).Debug("Enqueueing add container jobs")
	enqueueChan := make(chan queue.Job, len(containers))
	go func() {
		for addJob := range enqueueChan {
			job.queue.Enqueue(addJob)
		}
	}()
	for _, container := range containers {
		enqueueChan <- NewAddContainerJob(
			container.ID,
			job.containerStore,
			job.credentialStore,
		)
	}
	return nil
}
