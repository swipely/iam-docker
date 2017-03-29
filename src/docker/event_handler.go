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
	wlog := log.WithField("event-handler", workerID)
	wlog.Info("Starting event handler")
	for event := range channel {
		if (event.Status != "start") && (event.Status != "die") {
			continue
		}
		elog := wlog.WithFields(logrus.Fields{
			"id":    event.ID,
			"event": event.Status,
		})
		elog.Debug("Handling event")
		if event.Status == "start" {
			elog.Info("Adding container")
			err := handler.containerStore.AddContainerByID(event.ID)
			if err != nil {
				elog.WithField("error", err.Error()).Warn("Unable to add container")
				continue
			}
			role, err := handler.containerStore.IAMRoleForID(event.ID)
			if err != nil {
				elog.WithField("error", err.Error()).Warn("Unable to lookup IAM role")
				continue
			}
			rlog := elog.WithFields(logrus.Fields{"role": role})
			rlog.Info("Fetching credentials")
			_, err = handler.credentialStore.CredentialsForRole(role.Arn, role.ExternalId)
			if err != nil {
				rlog.WithField("error", err.Error()).Warn("Unable fetch credentials")
				continue
			}
		} else {
			elog.Info("Removing container")
			handler.containerStore.RemoveContainer(event.ID)
		}
	}
	wlog.Warn("Docker events channel closed")
}

type eventHandler struct {
	containerStore  ContainerStore
	credentialStore iam.CredentialStore
	workers         int
}
