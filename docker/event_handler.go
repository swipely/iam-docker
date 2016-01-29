package docker

import (
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	dockerClient "github.com/fsouza/go-dockerclient"
	"strings"
	"sync"
)

const (
	iamPrefix               = "IAM_PROFILE="
	dockerEventsChannelSize = 1000
)

var (
	runningContainersOpts = dockerClient.ListContainersOptions{
		Filters: map[string][]string{
			"status=": []string{"running"},
		},
	}
)

// NewContainerStoreEventHandler a new event handler that updates the container
// store based on Docker event updates. It requires a handle on the
// dockerClient.Client as well to retrieve metadata about added containers.
func NewContainerStoreEventHandler(store ContainerStore, client RawClient) EventHandler {
	return &containerStoreEventHandler{
		store:               store,
		client:              client,
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
	var writeGroup sync.WaitGroup

	handler.listenMutex.Lock()
	defer handler.listenMutex.Unlock()

	for event := range handler.DockerEventsChannel() {
		if event == nil {
			continue
		}
		id := event.ID
		switch event.Status {
		case "start":
			writeGroup.Add(1)
			go func() {
				handler.addContainerWithLogging(id)
				writeGroup.Done()
			}()
		case "die":
			writeGroup.Add(1)
			go func() {
				log.Info("Removing container ID: ", id)
				handler.store.RemoveContainer(id)
				writeGroup.Done()
			}()
		}
	}

	handler.dockerEventsChannel = nil
	writeGroup.Wait()

	return errors.New("Docker events connection closed")
}

func (handler *containerStoreEventHandler) SyncRunningContainers() error {
	log.Info("Syncing the running containers")
	var writeGroup sync.WaitGroup

	handler.store.Reset()
	apiContainers, err := handler.client.ListContainers(runningContainersOpts)
	if err != nil {
		_ = log.Warn("Error listing containers: ", err.Error())
		return err
	}

	writeGroup.Add(len(apiContainers))
	for _, apiContainer := range apiContainers {
		id := apiContainer.ID
		go func() {
			handler.addContainerWithLogging(id)
			writeGroup.Done()
		}()
	}

	writeGroup.Wait()
	log.Info("Successfully synced running contaniers")

	return nil
}

func (handler *containerStoreEventHandler) addContainerWithLogging(id string) {
	log.Info("Adding container ", id)
	err := handler.addContainer(id)
	if err == nil {
		log.Info("Successfully added container ", id)
	} else {
		_ = log.Warn("Unable to add container ", id, ": ", err.Error())
	}
}

func (handler *containerStoreEventHandler) addContainer(id string) error {
	container, err := handler.client.InspectContainer(id)
	if err != nil {
		return err
	} else if container == nil {
		return fmt.Errorf("Cannot inspect container: %s", id)
	} else if container.Config == nil {
		return fmt.Errorf("Container has no config: %s", id)
	} else if container.NetworkSettings == nil {
		return fmt.Errorf("Container has no network settings: %s", id)
	}

	role, err := findIAMRole(container.Config.Env)
	if err != nil {
		return err
	}
	ip := container.NetworkSettings.IPAddress

	handler.store.AddContainer(id, ip, role)

	return nil
}

func findIAMRole(env []string) (string, error) {
	if env != nil {
		for _, element := range env {
			if strings.HasPrefix(element, iamPrefix) {
				return strings.TrimPrefix(element, iamPrefix), nil
			}
		}
	}

	return "", fmt.Errorf("Unable to find environment variable with prefix: %s", iamPrefix)
}

type containerStoreEventHandler struct {
	store               ContainerStore
	client              RawClient
	dockerEventsChannel *(chan *dockerClient.APIEvents)
	listenMutex         sync.Mutex
}
