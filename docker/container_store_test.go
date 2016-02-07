package docker_test

import (
	dockerClient "github.com/fsouza/go-dockerclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/swipely/iam-docker/docker"
	"sort"
)

type mockClient struct {
	containersByID map[string]*dockerClient.Container
}

func newMockClient() *mockClient {
	return &mockClient{
		containersByID: make(map[string]*dockerClient.Container),
	}
}

func (mock *mockClient) InspectContainer(id string) (*dockerClient.Container, error) {
	container, hasKey := mock.containersByID[id]
	if !hasKey {
		return nil, &dockerClient.NoSuchContainer{ID: id}
	}
	return container, nil
}

func (mock *mockClient) ListContainers(opts dockerClient.ListContainersOptions) ([]dockerClient.APIContainers, error) {
	containers := make([]dockerClient.APIContainers, len(mock.containersByID))
	count := 0
	for id := range mock.containersByID {
		containers[count] = dockerClient.APIContainers{ID: id}
		count++
	}
	return containers, nil
}

var _ = Describe("ContainerStore", func() {
	var (
		client  *mockClient
		subject ContainerStore
	)

	BeforeEach(func() {
		client = newMockClient()
		subject = NewContainerStore(client)
	})

	Describe("AddContainerByID", func() {
		const (
			id = "DEADBEEF"
		)

		Context("When the container cannot be found", func() {
			It("Does not add the container to the store", func() {
				err := subject.AddContainerByID(id)
				Expect(err).ToNot(BeNil())
				role, err := subject.IAMRoleForID(id)
				Expect(role).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When the container can be found", func() {
			const (
				ip = "172.0.0.2"
			)

			BeforeEach(func() {
				client.containersByID[id] = &dockerClient.Container{
					ID:              id,
					Config:          &dockerClient.Config{Env: []string{}},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: ip},
				}
			})

			Context("But it does not have an IAM role set", func() {
				It("Does not add the container to the store", func() {
					err := subject.AddContainerByID(id)
					Expect(err).ToNot(BeNil())
					role, err := subject.IAMRoleForID(id)
					Expect(role).To(Equal(""))
					Expect(err).ToNot(BeNil())
				})
			})

			Context("And it has an IAM role set", func() {
				const (
					role = "arn:aws:iam::012345678901:role/test"
				)

				BeforeEach(func() {
					client.containersByID[id].Config.Env = []string{"IAM_PROFILE=" + role}
				})

				It("Adds the container to the store", func() {
					err := subject.AddContainerByID(id)
					Expect(err).To(BeNil())
					actual, err := subject.IAMRoleForID(id)
					Expect(actual).To(Equal(role))
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe("IAMRoles", func() {
		var (
			roles = []string{"arn:aws:iam::012345678901:role/alpha", "arn:aws:iam::012345678901:role/beta"}
		)

		BeforeEach(func() {
			client.containersByID["DEADBEEF"] = &dockerClient.Container{
				ID:              "DEADBEEF",
				Config:          &dockerClient.Config{Env: []string{"IAM_PROFILE=arn:aws:iam::012345678901:role/alpha"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.2"},
			}
			client.containersByID["FEEDABEE"] = &dockerClient.Container{
				ID:              "FEEDABEE",
				Config:          &dockerClient.Config{Env: []string{"IAM_PROFILE=arn:aws:iam::012345678901:role/beta"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.3"},
			}
			client.containersByID["CA55E77E"] = &dockerClient.Container{
				ID:              "CA55E77E",
				Config:          &dockerClient.Config{Env: []string{"IAM_PROFILE=arn:aws:iam::012345678901:role/alpha"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.4"},
			}
			_ = subject.SyncRunningContainers()
		})

		It("Returns the IAM roles that are stored", func() {
			actual := subject.IAMRoles()
			sort.Strings(actual)
			sort.Strings(roles)
			Expect(actual).To(Equal(roles))
		})
	})

	Describe("IAMRoleForID", func() {
		const (
			id = "80858085"
		)

		Context("When the ID is not stored", func() {
			It("Returns an error", func() {
				actual, err := subject.IAMRoleForID(id)
				Expect(actual).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When the ID is stored", func() {
			const (
				role = "arn:aws:iam::012345678901:role/dynamo"
			)

			BeforeEach(func() {
				client.containersByID[id] = &dockerClient.Container{
					ID:              id,
					Config:          &dockerClient.Config{Env: []string{"IAM_PROFILE=" + role}},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.2"},
				}
				_ = subject.SyncRunningContainers()
			})

			It("Returns the IAM role", func() {
				actual, err := subject.IAMRoleForID(id)
				Expect(actual).To(Equal(role))
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("IAMRoleForIP", func() {
		const (
			id   = "CA75EA75"
			ip   = "172.0.0.99"
			role = "arn:aws:iam::012345678901:role/s3-rw"
		)

		Context("When the IP is not stored", func() {
			It("Returns an error", func() {
				actual, err := subject.IAMRoleForIP(ip)
				Expect(actual).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When the IP is stored", func() {
			BeforeEach(func() {
				client.containersByID[id] = &dockerClient.Container{
					ID:              id,
					Config:          &dockerClient.Config{Env: []string{"IAM_PROFILE=" + role}},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: ip},
				}
				_ = subject.SyncRunningContainers()
			})

			It("Returns the IAM role", func() {
				actual, err := subject.IAMRoleForIP(ip)
				Expect(actual).To(Equal(role))
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("RemoveContainer", func() {
		const (
			id   = "BEA72A55"
			ip   = "172.0.0.52"
			role = "arn:aws:iam::012345678901:role/cloudwatch"
		)

		Context("When the ID is not stored", func() {
			It("Does not change the store", func() {
				actual, err := subject.IAMRoleForID(id)
				Expect(actual).To(Equal(""))
				Expect(err).ToNot(BeNil())
				subject.RemoveContainer(id)
				actual, err = subject.IAMRoleForID(id)
				Expect(actual).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When the ID is stored", func() {
			BeforeEach(func() {
				client.containersByID[id] = &dockerClient.Container{
					ID:              id,
					Config:          &dockerClient.Config{Env: []string{"IAM_PROFILE=" + role}},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: ip},
				}
				_ = subject.SyncRunningContainers()
			})

			It("Removes the container", func() {
				actual, err := subject.IAMRoleForID(id)
				Expect(actual).To(Equal(role))
				Expect(err).To(BeNil())
				subject.RemoveContainer(id)
				actual, err = subject.IAMRoleForID(id)
				Expect(actual).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("SyncRunningContainers", func() {

		BeforeEach(func() {
			client.containersByID["38BE1290"] = &dockerClient.Container{
				ID:              "38BE1290",
				Config:          &dockerClient.Config{Env: []string{"IAM_PROFILE=arn:aws:iam::012345678901:role/reader"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.15"},
			}
			client.containersByID["EF10A722"] = &dockerClient.Container{
				ID:              "EF10A722",
				Config:          &dockerClient.Config{Env: []string{"IAM_PROFILE=arn:aws:iam::012345678901:role/writer"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.16"},
			}
			client.containersByID["F00DF00D"] = &dockerClient.Container{
				ID:              "F00DF00D",
				Config:          &dockerClient.Config{Env: []string{}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.17"},
			}
		})

		It("Adds the containers which have IAM roles set", func() {
			err := subject.SyncRunningContainers()
			Expect(err).To(BeNil())
			role, err := subject.IAMRoleForIP("172.0.0.15")
			Expect(role).To(Equal("arn:aws:iam::012345678901:role/reader"))
			Expect(err).To(BeNil())
			role, err = subject.IAMRoleForIP("172.0.0.16")
			Expect(role).To(Equal("arn:aws:iam::012345678901:role/writer"))
			Expect(err).To(BeNil())
			role, err = subject.IAMRoleForIP("172.0.0.17")
			Expect(role).To(Equal(""))
			Expect(err).ToNot(BeNil())
		})
	})
})
