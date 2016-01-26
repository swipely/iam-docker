package docker_test

import (
	"fmt"
	dockerClient "github.com/fsouza/go-dockerclient"
	. "github.com/swipely/iam-docker/docker"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MockDockerClient struct {
	inspectContainer func(id string) (*dockerClient.Container, error)
}

func (client *MockDockerClient) InspectContainer(id string) (*dockerClient.Container, error) {
	if client.inspectContainer == nil {
		return nil, fmt.Errorf("Unable to inspect container because mock was not set: %s", id)
	}
	container, err := client.inspectContainer(id)
	return container, err

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
})
