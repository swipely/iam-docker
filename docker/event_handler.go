package docker

import (
	"errors"
	dockerClient "github.com/fsouza/go-dockerclient"
	iam "github.com/swipely/iam-docker/iam"
	"sync"
)

const (
	concurrentEventHandlers = 4
	dockerEventsChannelSize = 1000
)

// NewEventHandler a new event handler that updates the container and IAM stores
// based on Docker event updates.
func NewEventHandler(containerStore ContainerStore, credentialStore iam.CredentialStore) EventHandler {
	return &eventHandler{
		containerStore:      containerStore,
		credentialStore:     credentialStore,
		dockerEventsChannel: nil,
	}
}

func (handler *eventHandler) DockerEventsChannel() chan *dockerClient.APIEvents {
	if handler.dockerEventsChannel == nil {
		channel := make(chan *dockerClient.APIEvents, dockerEventsChannelSize)
		handler.dockerEventsChannel = &channel
	}
	return *handler.dockerEventsChannel
}

func (handler *eventHandler) Listen() error {
	var workers sync.WaitGroup
	channel := handler.DockerEventsChannel()

	handler.listenMutex.Lock()
	defer handler.listenMutex.Unlock()

	workers.Add(concurrentEventHandlers)
	for i := 0; i < concurrentEventHandlers; i++ {
		go func() {
			for event := range channel {
				id := event.ID
				switch event.Status {
				case "start":
					err := handler.containerStore.AddContainerByID(id)
					if err == nil {
						role, err := handler.containerStore.IAMRoleForID(id)
						if err == nil {
							_, _ = handler.credentialStore.CredentialsForRole(role)
						}
					}
				case "die":
					handler.containerStore.RemoveContainer(id)
				}
			}
			workers.Done()
		}()
	}

	workers.Wait()
	handler.dockerEventsChannel = nil

	return errors.New("Docker events connection closed")
}

type eventHandler struct {
	containerStore      ContainerStore
	credentialStore     iam.CredentialStore
	dockerEventsChannel *(chan *dockerClient.APIEvents)
	listenMutex         sync.Mutex
}
