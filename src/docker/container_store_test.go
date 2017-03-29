package docker_test

import (
	dockerClient "github.com/fsouza/go-dockerclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/mock"
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
					ID:     id,
					Config: &dockerClient.Config{Labels: map[string]string{}},
					NetworkSettings: &dockerClient.NetworkSettings{
						Networks: map[string]dockerClient.ContainerNetwork{
							"bridge": dockerClient.ContainerNetwork{
								IPAddress: ip,
							},
						},
					},
				})
				Expect(err).To(BeNil())
			})

			It("Does not add the container to the store", func() {
				err := subject.AddContainerByID(id)
				Expect(err).ToNot(BeNil())
				role, err := subject.IAMRoleForID(id)
				Expect(role.Arn).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("And it has an IAM role set via label", func() {
			const (
				role = "arn:aws:iam::012345678901:role/test"
			)

			Context("But it does not have an IP set", func() {
				BeforeEach(func() {
					err := client.AddContainer(&dockerClient.Container{
						ID: id,
						Config: &dockerClient.Config{
							Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role},
						},
						NetworkSettings: &dockerClient.NetworkSettings{
							Networks: map[string]dockerClient.ContainerNetwork{
								"bridge": dockerClient.ContainerNetwork{
									IPAddress: "",
								},
							},
						},
					})
					Expect(err).To(BeNil())
				})

				It("Does not add the container to the store", func() {
					err := subject.AddContainerByID(id)
					Expect(err).ToNot(BeNil())
					role, err := subject.IAMRoleForID(id)
					Expect(role.Arn).To(Equal(""))
					Expect(err).ToNot(BeNil())
				})
			})

			Context("And it has an IP set", func() {
				BeforeEach(func() {
					err := client.AddContainer(&dockerClient.Container{
						ID: id,
						Config: &dockerClient.Config{
							Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role},
						},
						NetworkSettings: &dockerClient.NetworkSettings{
							Networks: map[string]dockerClient.ContainerNetwork{
								"bridge": dockerClient.ContainerNetwork{
									IPAddress: ip,
								},
							},
						},
					})
					Expect(err).To(BeNil())
				})

				It("Adds the container to the store", func() {
					err := subject.AddContainerByID(id)
					Expect(err).To(BeNil())
					actual, err := subject.IAMRoleForID(id)
					Expect(actual.Arn).To(Equal(role))
					Expect(err).To(BeNil())
				})
			})
		})

		Context("And it has an IAM role set via environment variable", func() {
			const (
				role = "arn:aws:iam::012345678901:role/test"
			)

			Context("But it does not have an IP set", func() {
				BeforeEach(func() {
					err := client.AddContainer(&dockerClient.Container{
						ID: id,
						Config: &dockerClient.Config{
							Labels: map[string]string{},
							Env:    []string{"IAM_ROLE=" + role},
						},
						NetworkSettings: &dockerClient.NetworkSettings{
							Networks: map[string]dockerClient.ContainerNetwork{
								"bridge": dockerClient.ContainerNetwork{
									IPAddress: "",
								},
							},
						},
					})
					Expect(err).To(BeNil())
				})

				It("Does not add the container to the store", func() {
					err := subject.AddContainerByID(id)
					Expect(err).ToNot(BeNil())
					role, err := subject.IAMRoleForID(id)
					Expect(role.Arn).To(Equal(""))
					Expect(err).ToNot(BeNil())
				})
			})

			Context("And it has an IP set", func() {
				BeforeEach(func() {
					err := client.AddContainer(&dockerClient.Container{
						ID: id,
						Config: &dockerClient.Config{
							Labels: map[string]string{},
							Env:    []string{"IAM_ROLE=" + role},
						},
						NetworkSettings: &dockerClient.NetworkSettings{
							Networks: map[string]dockerClient.ContainerNetwork{
								"bridge": dockerClient.ContainerNetwork{
									IPAddress: ip,
								},
							},
						},
					})
					Expect(err).To(BeNil())
				})

				It("Adds the container to the store", func() {
					err := subject.AddContainerByID(id)
					Expect(err).To(BeNil())
					actual, err := subject.IAMRoleForID(id)
					Expect(actual.Arn).To(Equal(role))
					Expect(err).To(BeNil())
				})
			})
		})
	})

	Describe("IAMRoles", func() {
		var (
			roles = []ComplexRole{
				ComplexRole{
					Arn:        "arn:aws:iam::012345678901:role/alpha",
					ExternalId: "",
				},
				ComplexRole{
					Arn:        "arn:aws:iam::012345678901:role/beta",
					ExternalId: "",
				},
			}
		)

		BeforeEach(func() {
			_ = client.AddContainer(&dockerClient.Container{
				ID:     "DEADBEEF",
				Config: &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/alpha"}},
				NetworkSettings: &dockerClient.NetworkSettings{
					Networks: map[string]dockerClient.ContainerNetwork{
						"bridge": dockerClient.ContainerNetwork{
							IPAddress: "172.0.0.2",
						},
					},
				},
			})
			_ = client.AddContainer(&dockerClient.Container{
				ID:     "FEEDABEE",
				Config: &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/beta"}},
				NetworkSettings: &dockerClient.NetworkSettings{
					Networks: map[string]dockerClient.ContainerNetwork{
						"bridge": dockerClient.ContainerNetwork{
							IPAddress: "172.0.0.3",
						},
					},
				},
			})
			_ = client.AddContainer(&dockerClient.Container{
				ID:     "CA55E77E",
				Config: &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/alpha"}},
				NetworkSettings: &dockerClient.NetworkSettings{
					Networks: map[string]dockerClient.ContainerNetwork{
						"bridge": dockerClient.ContainerNetwork{
							IPAddress: "172.0.0.4",
						},
					},
				},
			})
			_ = subject.SyncRunningContainers()
		})

		It("Returns the IAM roles that are stored", func() {
			actual := subject.IAMRoles()
			Expect(len(actual)).To(Equal(len(roles)))
		})
	})

	Describe("IAMRoleForID", func() {
		const (
			id = "80858085"
		)

		Context("When the ID is not stored", func() {
			It("Returns an error", func() {
				actual, err := subject.IAMRoleForID(id)
				Expect(actual.Arn).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When the ID is stored", func() {
			const (
				role = "arn:aws:iam::012345678901:role/dynamo"
			)

			BeforeEach(func() {
				_ = client.AddContainer(&dockerClient.Container{
					ID:     id,
					Config: &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role}},
					NetworkSettings: &dockerClient.NetworkSettings{
						Networks: map[string]dockerClient.ContainerNetwork{
							"bridge": dockerClient.ContainerNetwork{
								IPAddress: "172.0.0.2",
							},
						},
					},
				})
				_ = subject.SyncRunningContainers()
			})

			It("Returns the IAM role", func() {
				actual, err := subject.IAMRoleForID(id)
				Expect(actual.Arn).To(Equal(role))
				Expect(err).To(BeNil())
			})
		})
	})

	Describe("IAMRoleForIP", func() {
		const (
			id    = "CA75EA75"
			ipOne = "172.0.0.99"
			ipTwo = "173.0.0.98"
			role  = "arn:aws:iam::012345678901:role/s3-rw"
		)

		Context("When the IP is not stored", func() {
			It("Returns an error", func() {
				actual, err := subject.IAMRoleForIP(ipOne)
				Expect(actual.Arn).To(Equal(""))
				Expect(err).ToNot(BeNil())
				actual, err = subject.IAMRoleForIP(ipTwo)
				Expect(actual.Arn).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When the IP is stored", func() {
			BeforeEach(func() {
				_ = client.AddContainer(&dockerClient.Container{
					ID:     id,
					Config: &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role}},
					NetworkSettings: &dockerClient.NetworkSettings{
						Networks: map[string]dockerClient.ContainerNetwork{
							"bridge": dockerClient.ContainerNetwork{
								IPAddress: "172.0.0.99",
							},
							"other": dockerClient.ContainerNetwork{
								IPAddress: "173.0.0.98",
							},
						},
					},
				})
				_ = subject.SyncRunningContainers()
			})

			It("Returns the IAM role", func() {
				actual, err := subject.IAMRoleForIP(ipOne)
				Expect(actual.Arn).To(Equal(role))
				Expect(err).To(BeNil())
				actual, err = subject.IAMRoleForIP(ipTwo)
				Expect(actual.Arn).To(Equal(role))
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
				Expect(actual.Arn).To(Equal(""))
				Expect(err).ToNot(BeNil())
				subject.RemoveContainer(id)
				actual, err = subject.IAMRoleForID(id)
				Expect(actual.Arn).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})

		Context("When the ID is stored", func() {
			BeforeEach(func() {
				_ = client.AddContainer(&dockerClient.Container{
					ID:     id,
					Config: &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role}},
					NetworkSettings: &dockerClient.NetworkSettings{
						Networks: map[string]dockerClient.ContainerNetwork{
							"bridge": dockerClient.ContainerNetwork{
								IPAddress: ip,
							},
						},
					},
				})
				_ = subject.SyncRunningContainers()
			})

			It("Removes the container", func() {
				actual, err := subject.IAMRoleForID(id)
				Expect(actual.Arn).To(Equal(role))
				Expect(err).To(BeNil())
				subject.RemoveContainer(id)
				actual, err = subject.IAMRoleForID(id)
				Expect(actual.Arn).To(Equal(""))
				Expect(err).ToNot(BeNil())
			})
		})
	})

	Describe("SyncRunningContainers", func() {
		BeforeEach(func() {
			_ = client.AddContainer(&dockerClient.Container{
				ID:     "38BE1290",
				Config: &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/reader"}},
				NetworkSettings: &dockerClient.NetworkSettings{
					Networks: map[string]dockerClient.ContainerNetwork{
						"bridge": dockerClient.ContainerNetwork{
							IPAddress: "172.0.0.15",
						},
					},
				},
			})
			_ = client.AddContainer(&dockerClient.Container{
				ID:     "EF10A722",
				Config: &dockerClient.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": "arn:aws:iam::012345678901:role/writer", "com.swipely.iam-docker.iam-externalid": "eid"}},
				NetworkSettings: &dockerClient.NetworkSettings{
					Networks: map[string]dockerClient.ContainerNetwork{
						"bridge": dockerClient.ContainerNetwork{
							IPAddress: "172.0.0.16",
						},
					},
				},
			})
			_ = client.AddContainer(&dockerClient.Container{
				ID:     "F00DF00D",
				Config: &dockerClient.Config{Labels: map[string]string{}},
				NetworkSettings: &dockerClient.NetworkSettings{
					Networks: map[string]dockerClient.ContainerNetwork{
						"bridge": dockerClient.ContainerNetwork{
							IPAddress: "172.0.0.17",
						},
					},
				},
			})
		})

		It("Adds the containers which have IAM roles set", func() {
			err := subject.SyncRunningContainers()
			Expect(err).To(BeNil())
			role, err := subject.IAMRoleForIP("172.0.0.15")
			Expect(role.Arn).To(Equal("arn:aws:iam::012345678901:role/reader"))
			Expect(role.ExternalId).To(Equal(""))
			Expect(err).To(BeNil())
			role, err = subject.IAMRoleForIP("172.0.0.16")
			Expect(role.Arn).To(Equal("arn:aws:iam::012345678901:role/writer"))
			Expect(role.ExternalId).To(Equal("eid"))
			Expect(err).To(BeNil())
			role, err = subject.IAMRoleForIP("172.0.0.17")
			Expect(role.Arn).To(Equal(""))
			Expect(err).ToNot(BeNil())
		})
	})
})
