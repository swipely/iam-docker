package docker

import (
	"errors"
	"github.com/Sirupsen/logrus"
	dockerClient "github.com/fsouza/go-dockerclient"
	iam "github.com/swipely/iam-docker/src/iam"
	"sync"
)

// NewEventHandler a new event handler that updates the container and IAM stores
// based on Docker event updates.
func NewEventHandler(workers int, containerStore ContainerStore, credentialStore iam.CredentialStore) EventHandler {
	return &eventHandler{
		workers:         workers,
		containerStore:  containerStore,
		credentialStore: credentialStore,
	}
}

func (handler *eventHandler) Listen(channel <-chan *dockerClient.APIEvents) error {
	var workers sync.WaitGroup

	workers.Add(handler.workers)
	for i := 1; i <= handler.workers; i++ {
		id := i
		go func() {
			handler.work(id, channel)
			workers.Done()
		}()
	}
	workers.Wait()

	return errors.New("Docker events connection closed")
}

func (handler *eventHandler) work(workerID int, channel <-chan *dockerClient.APIEvents) {
	wlog := log.WithFields(logrus.Fields{"worker": workerID})
	wlog.Info("Starting worker")
	for event := range channel {
		if (event.Status != "start") && (event.Status != "die") {
			continue
		}
		elog := wlog.WithFields(logrus.Fields{
			"id":    event.ID,
			"event": event.Status,
		})
		elog.Info("Handling event")
		if event.Status == "start" {
			err := handler.containerStore.AddContainerByID(event.ID)
			if err != nil {
				elog.WithFields(logrus.Fields{
					"error": err.Error(),
				}).Warn("Unable to add container")
				continue
			}
			role, err := handler.containerStore.IAMRoleForID(event.ID)
			if err != nil {
				elog.WithFields(logrus.Fields{
					"error": err.Error(),
				}).Warn("Unable to lookup IAM role")
				continue
			}
			_, err = handler.credentialStore.CredentialsForRole(role)
			elog = elog.WithFields(logrus.Fields{"role": role})
			if err != nil {
				elog.WithFields(logrus.Fields{
					"error": err.Error(),
				}).Warn("Unable fetch credentials")
				continue
			}
			elog.Info("Successfully fetched credentials")
		} else {
			handler.containerStore.RemoveContainer(event.ID)
		}
	}
	wlog.Info("Work complete")
}

type eventHandler struct {
	containerStore  ContainerStore
	credentialStore iam.CredentialStore
	workers         int
}
