package docker_test

import (
	"errors"
	"fmt"
	dockerClient "github.com/fsouza/go-dockerclient"
	. "github.com/swipely/iam-docker/docker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockDockerClient struct {
	inspectContainer func(id string) (*dockerClient.Container, error)
	listContainers   func(opts dockerClient.ListContainersOptions) ([]dockerClient.APIContainers, error)
}

func (client *MockDockerClient) InspectContainer(id string) (*dockerClient.Container, error) {
	if client.inspectContainer == nil {
		return nil, fmt.Errorf("Unable to inspect container because mock was not set: %s", id)
	}
	container, err := client.inspectContainer(id)
	return container, err

}

func (client *MockDockerClient) ListContainers(opts dockerClient.ListContainersOptions) ([]dockerClient.APIContainers, error) {
	if client.listContainers == nil {
		return nil, errors.New("unable to list containers because mock was not set")
	}
	containers, err := client.listContainers(opts)
	return containers, err

}

var _ = Describe("EventHandler", func() {
	var (
		client  *MockDockerClient
		store   ContainerStore
		handler EventHandler
	)

	BeforeEach(func() {
		client = &MockDockerClient{}
		store = NewContainerStore()
		handler = NewContainerStoreEventHandler(store, client)
	})

	Describe("Listen", func() {
		const (
			id      = "CA55E77E"
			iamRole = "arn:aws:iam::0123456789:role/test-role"
			ip      = "172.0.0.3"
		)
		var (
			event *dockerClient.APIEvents
		)

		Context("When a start event is passed down the channel", func() {
			BeforeEach(func() {
				event = &dockerClient.APIEvents{
					Status: "start",
					ID:     id,
				}
			})

			Context("When the container has all of the attributes", func() {
				JustBeforeEach(func() {
					client.inspectContainer = func(given string) (*dockerClient.Container, error) {
						if given != id {
							return nil, fmt.Errorf("No such container: %s", given)
						}
						container := &dockerClient.Container{
							Config: &dockerClient.Config{
								Env: []string{"IAM_PROFILE=" + iamRole},
							},
							NetworkSettings: &dockerClient.NetworkSettings{
								IPAddress: ip,
							},
						}
						return container, nil
					}
				})

				It("Adds that container to the store", func() {
					handler.DockerEventsChannel() <- event
					close(handler.DockerEventsChannel())
					_ = handler.Listen()
					Expect(store.IAMRoleForIP(ip)).To(Equal(iamRole))
				})
			})

			Context("When the container cannot be found", func() {
				JustBeforeEach(func() {
					client.inspectContainer = func(given string) (*dockerClient.Container, error) {
						return nil, fmt.Errorf("No such container: %s", given)
					}
				})

				It("Does not add that container to the store", func() {
					handler.DockerEventsChannel() <- event
					close(handler.DockerEventsChannel())
					_ = handler.Listen()
					role, err := store.IAMRoleForIP(ip)
					Expect(role).To(Equal(""))
					Expect(err).ToNot(BeNil())
				})
			})

			Context("When the container has no IAM role", func() {
				JustBeforeEach(func() {
					client.inspectContainer = func(given string) (*dockerClient.Container, error) {
						if given != id {
							return nil, fmt.Errorf("No such container: %s", given)
						}
						container := &dockerClient.Container{
							Config: &dockerClient.Config{
								Env: []string{"YOU_ARE_PROFILE=" + iamRole},
							},
							NetworkSettings: &dockerClient.NetworkSettings{
								IPAddress: ip,
							},
						}
						return container, nil
					}
				})

				It("Does not add that container to the store", func() {
					handler.DockerEventsChannel() <- event
					close(handler.DockerEventsChannel())
					_ = handler.Listen()
					role, err := store.IAMRoleForIP(ip)
					Expect(role).To(Equal(""))
					Expect(err).ToNot(BeNil())
				})
			})
		})

		Context("When a die event is passed down the channel", func() {
			BeforeEach(func() {
				event = &dockerClient.APIEvents{
					Status: "die",
					ID:     id,
				}
			})

			Context("When the ID is not in the store", func() {
				It("Changes nothing", func() {
					role, err := store.IAMRoleForIP(ip)
					Expect(role).To(Equal(""))
					Expect(err).ToNot(BeNil())
					handler.DockerEventsChannel() <- event
					close(handler.DockerEventsChannel())
					_ = handler.Listen()
					role, err = store.IAMRoleForIP(ip)
					Expect(role).To(Equal(""))
					Expect(err).ToNot(BeNil())
				})
			})

			Context("When the is in the store", func() {
				BeforeEach(func() {
					store.AddContainer(id, ip, iamRole)
				})

				It("Deletes the ID", func() {
					Expect(store.IAMRoleForIP(ip)).To(Equal(iamRole))
					handler.DockerEventsChannel() <- event
					close(handler.DockerEventsChannel())
					_ = handler.Listen()
					role, err := store.IAMRoleForIP(ip)
					Expect(role).To(Equal(""))
					Expect(err).ToNot(BeNil())
				})
			})
		})
	})

	Describe("SyncRunningContainers", func() {
		Context("When there is an error listing the running containers", func() {
			JustBeforeEach(func() {
				client.listContainers = func(opts dockerClient.ListContainersOptions) ([]dockerClient.APIContainers, error) {
					return nil, errors.New("Error communicating with Docker")
				}
			})

			It("Raises an error", func() {
				err := handler.SyncRunningContainers()
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When listing the running containers succeeds", func() {
			var (
				apiContainers []dockerClient.APIContainers
			)

			JustBeforeEach(func() {
				apiContainers = []dockerClient.APIContainers{
					dockerClient.APIContainers{ID: "DEADBEEF"},
					dockerClient.APIContainers{ID: "CA55E77E"},
				}

				client.listContainers = func(opts dockerClient.ListContainersOptions) ([]dockerClient.APIContainers, error) {
					return apiContainers, nil
				}
				client.inspectContainer = func(id string) (*dockerClient.Container, error) {
					if id == "DEADBEEF" {
						container := &dockerClient.Container{
							Config: &dockerClient.Config{
								Env: []string{"IAM_PROFILE=test-iam-role"},
							},
							NetworkSettings: &dockerClient.NetworkSettings{
								IPAddress: "172.0.0.2",
							},
						}
						return container, nil
					} else if id == "CA55E77E" {
						container := &dockerClient.Container{
							Config: &dockerClient.Config{
								Env: []string{},
							},
							NetworkSettings: &dockerClient.NetworkSettings{
								IPAddress: "172.0.0.3",
							},
						}
						return container, nil
					}

					return nil, fmt.Errorf("No such container: %s", id)
				}
			})

			It("Adds the containers which have the necessary information", func() {
				err := handler.SyncRunningContainers()
				Expect(err).To(BeNil())
				role, err := store.IAMRoleForIP("172.0.0.2")
				Expect(role).To(Equal("test-iam-role"))
				Expect(err).To(BeNil())
				role, err = store.IAMRoleForIP("172.0.0.3")
				Expect(role).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})
	})
})
