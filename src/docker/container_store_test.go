package docker_test

import (
	dockerClient "github.com/fsouza/go-dockerclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/mock"
	"sort"
)

var _ = Describe("ContainerStore", func() {
	var (
		client  *mock.DockerClient
		subject ContainerStore
	)

	BeforeEach(func() {
		client = mock.NewDockerClient()
		subject = NewContainerStore(client)
	})

	Describe("AddContainerByID", func() {
		const (
			id = "DEADBEEF"
			ip = "172.0.0.2"
		)

		Context("But it does not have an IAM role set", func() {
			BeforeEach(func() {
				err := client.AddContainer(&dockerClient.Container{
					ID:              id,
					Config:          &dockerClient.Config{Labels: map[string]string{}},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: ip},
				})
				Expect(err).To(BeNil())
			})

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
				err := client.AddContainer(&dockerClient.Container{
					ID: id,
					Config: &dockerClient.Config{
						Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role},
					},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: ip},
				})
				Expect(err).To(BeNil())
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

	Describe("IAMRoles", func() {
		var (
			roles = []string{"arn:aws:iam::012345678901:role/alpha", "arn:aws:iam::012345678901:role/beta"}
		)

		BeforeEach(func() {
			_ = client.AddContainer(&dockerClient.Container{
				ID:              "DEADBEEF",
				Config:          &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/alpha"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.2"},
			})
			_ = client.AddContainer(&dockerClient.Container{
				ID:              "FEEDABEE",
				Config:          &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/beta"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.3"},
			})
			_ = client.AddContainer(&dockerClient.Container{
				ID:              "CA55E77E",
				Config:          &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/alpha"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.4"},
			})
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
				_ = client.AddContainer(&dockerClient.Container{
					ID:              id,
					Config:          &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role}},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.2"},
				})
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
				_ = client.AddContainer(&dockerClient.Container{
					ID:              id,
					Config:          &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role}},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: ip},
				})
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
				_ = client.AddContainer(&dockerClient.Container{
					ID:              id,
					Config:          &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role}},
					NetworkSettings: &dockerClient.NetworkSettings{IPAddress: ip},
				})
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
			_ = client.AddContainer(&dockerClient.Container{
				ID:              "38BE1290",
				Config:          &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/reader"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.15"},
			})
			_ = client.AddContainer(&dockerClient.Container{
				ID:              "EF10A722",
				Config:          &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/writer"}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.16"},
			})
			_ = client.AddContainer(&dockerClient.Container{
				ID:              "F00DF00D",
				Config:          &dockerClient.Config{Labels: map[string]string{}},
				NetworkSettings: &dockerClient.NetworkSettings{IPAddress: "172.0.0.17"},
			})
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
