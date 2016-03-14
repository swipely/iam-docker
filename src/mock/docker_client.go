package mock

import (
	docker "github.com/fsouza/go-dockerclient"
)

// DockerClient implements the
// github.com/swipely/iam-docker/src/docker.RawClient interface. To fake a
// running container, it must be added to the containersByID map.
type DockerClient struct {
	containersByID map[string]*docker.Container
	eventListeners []chan<- *docker.APIEvents
}

// NewDockerClient creates a new mock Docker client.
func NewDockerClient() *DockerClient {
	return &DockerClient{
		containersByID: make(map[string]*docker.Container),
		eventListeners: make([]chan<- *docker.APIEvents, 0),
	}
}

// AddEventListener is a no-op.
func (mock *DockerClient) AddEventListener(channel chan<- *docker.APIEvents) error {
	mock.eventListeners = append(mock.eventListeners, channel)
	return nil
}

// AddContainer adds the container the to the store and fires off the event
// listeners.
func (mock *DockerClient) AddContainer(container *docker.Container) error {
	_, hasKey := mock.containersByID[container.ID]
	if hasKey {
		return &docker.ContainerAlreadyRunning{ID: container.ID}
	}

	mock.containersByID[container.ID] = container
	mock.triggerListeners(&docker.APIEvents{
		ID:     container.ID,
		Status: "start",
	})

	return nil
}

// RemoveContainer removes the container and fires off event listeners.
func (mock *DockerClient) RemoveContainer(id string) error {
	_, hasKey := mock.containersByID[id]
	if !hasKey {
		return &docker.NoSuchContainer{ID: id}
	}

	delete(mock.containersByID, id)
	mock.triggerListeners(&docker.APIEvents{
		ID:     id,
		Status: "die",
	})

	return nil
}

// InspectContainer looks up a container by its ID.
func (mock *DockerClient) InspectContainer(id string) (*docker.Container, error) {
	container, hasKey := mock.containersByID[id]
	if !hasKey {
		return nil, &docker.NoSuchContainer{ID: id}
	}
	return container, nil
}

// ListContainers returns a docker.APIContainer for each container stored in the
// mock.
func (mock *DockerClient) ListContainers(opts docker.ListContainersOptions) ([]docker.APIContainers, error) {
	containers := make([]docker.APIContainers, len(mock.containersByID))
	count := 0
	for id := range mock.containersByID {
		containers[count] = docker.APIContainers{ID: id}
		count++
	}
	return containers, nil
}

func (mock *DockerClient) triggerListeners(event *docker.APIEvents) {
	for _, eventListener := range mock.eventListeners {
		eventListener <- event
	}
}
