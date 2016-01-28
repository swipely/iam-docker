package docker

import (
	dockerClient "github.com/fsouza/go-dockerclient"
)

// ContainerStore exposes methods to handle container lifecycle events.
// Instances of this interface should allow threadsafe reads and writes.
type ContainerStore interface {
	AddContainer(name string, ip string, iamRole string)
	RemoveContainer(name string)
	IAMRoleForIP(ip string) (string, error)
}

// EventHandler instances implement DockerEventsChannel() which performs actions
// based on Docker events. Listen() is a blocking function which performs an
// action based on the events written to the channel.
type EventHandler interface {
	DockerEventsChannel() chan *dockerClient.APIEvents
	Listen() error
}

// RawClient specifies the subset of commands that EventHandlers use from the
// go-dockerclient.
type RawClient interface {
	InspectContainer(id string) (*dockerClient.Container, error)
}
