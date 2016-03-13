package mock

import (
	docker "github.com/fsouza/go-dockerclient"
)

// DockerClient implements the
// github.com/swipely/iam-docker/src/docker.RawClient interface. To fake a
// running container, it must be added to the ContainersByID map.
type DockerClient struct {
	ContainersByID map[string]*docker.Container
}

// NewDockerClient creates a new mock Docker client.
func NewDockerClient() *DockerClient {
	return &DockerClient{
		ContainersByID: make(map[string]*docker.Container),
	}
}

// AddEventListener is a no-op.
func (mock *DockerClient) AddEventListener(chan<- docker.APIEvents) error {
	return nil
}

// InspectContainer looks up a container by its ID.
func (mock *DockerClient) InspectContainer(id string) (*docker.Container, error) {
	container, hasKey := mock.ContainersByID[id]
	if !hasKey {
		return nil, &docker.NoSuchContainer{ID: id}
	}
	return container, nil
}

// ListContainers returns a docker.APIContainer for each container stored in the
// mock.
func (mock *DockerClient) ListContainers(opts docker.ListContainersOptions) ([]docker.APIContainers, error) {
	containers := make([]docker.APIContainers, len(mock.ContainersByID))
	count := 0
	for id := range mock.ContainersByID {
		containers[count] = docker.APIContainers{ID: id}
		count++
	}
	return containers, nil
}
