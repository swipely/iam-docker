package docker

import (
	"errors"
	dockerClient "github.com/fsouza/go-dockerclient"
	"sync"
)

const (
	concurrentEventHandlers = 4
	dockerEventsChannelSize = 1000
)

// NewContainerStoreEventHandler a new event handler that updates the container
// store based on Docker event updates.
func NewContainerStoreEventHandler(store ContainerStore) EventHandler {
	return &containerStoreEventHandler{
		store:               store,
		dockerEventsChannel: nil,
	}
}

func (handler *containerStoreEventHandler) DockerEventsChannel() chan *dockerClient.APIEvents {
	if handler.dockerEventsChannel == nil {
		channel := make(chan *dockerClient.APIEvents, dockerEventsChannelSize)
		handler.dockerEventsChannel = &channel
	}
	return *handler.dockerEventsChannel
}

func (handler *containerStoreEventHandler) Listen() error {
	var workers sync.WaitGroup
	eventChan := make(chan *dockerClient.APIEvents, dockerEventsChannelSize)

	handler.listenMutex.Lock()
	defer handler.listenMutex.Unlock()

	workers.Add(concurrentEventHandlers)

	for i := 0; i < concurrentEventHandlers; i++ {
		go func() {
			for event := range eventChan {
				id := event.ID
				switch event.Status {
				case "start":
					_ = handler.store.AddContainerByID(id)
				case "die":
					handler.store.RemoveContainer(id)
				}
			}
			workers.Done()
		}()
	}

	for event := range handler.DockerEventsChannel() {
		if event != nil {
			eventChan <- event
		}
	}

	close(eventChan)
	workers.Wait()
	handler.dockerEventsChannel = nil

	return errors.New("Docker events connection closed")
}

type containerStoreEventHandler struct {
	store               ContainerStore
	dockerEventsChannel *(chan *dockerClient.APIEvents)
	listenMutex         sync.Mutex
}
