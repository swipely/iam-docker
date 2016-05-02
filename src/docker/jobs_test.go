package docker_test

import (
	"github.com/aws/aws-sdk-go/service/sts"
	dockerLib "github.com/fsouza/go-dockerclient"

	. "github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/iam"
	"github.com/swipely/iam-docker/src/mock"
	"github.com/swipely/iam-docker/src/queue"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Docker Jobs", func() {
	var (
		jobQueue        *mock.JobQueue
		dockerClient    *mock.DockerClient
		stsClient       *mock.STSClient
		containerStore  ContainerStore
		credentialStore iam.CredentialStore
		job             queue.Job
	)

	BeforeEach(func() {
		jobQueue = mock.NewJobQueue()
		dockerClient = mock.NewDockerClient()
		stsClient = mock.NewSTSClient()
		containerStore = NewContainerStore(dockerClient)
		credentialStore = iam.NewCredentialStore(stsClient, 1)
	})

	Describe("addContainerJob", func() {
		const (
			containerID = "deadbee"
		)

		JustBeforeEach(func() {
			job = NewAddContainerJob(containerID, containerStore, credentialStore)
		})

		Describe("ID", func() {
			It("Returns a unique identifier for the job", func() {
				Expect(job.ID()).To(Equal("docker/add-container/deadbee"))
			})
		})

		Describe("Perform", func() {
			Context("When the container cannot be inspected", func() {
				It("Fails", func() {
					err := job.Perform()
					Expect(err).ToNot(BeNil())
				})
			})

			Context("When the container can be inspected", func() {
				const (
					iamRole = "some-iam-role"
					ip      = "172.0.0.5"
				)

				JustBeforeEach(func() {
					err := dockerClient.AddContainer(&dockerLib.Container{
						ID: containerID,
						Config: &dockerLib.Config{
							Labels: map[string]string{
								"com.swipely.iam-docker.iam-profile": iamRole,
							},
						},
						NetworkSettings: &dockerLib.NetworkSettings{
							IPAddress: ip,
						},
					})
					Expect(err).To(BeNil())
				})

				Context("But the role cannot be assumed", func() {
					It("Fails", func() {
						err := job.Perform()
						Expect(err).ToNot(BeNil())
					})
				})

				Context("And the role can be assumed", func() {
					JustBeforeEach(func() {
						stsClient.AssumableRoles[iamRole] = &sts.Credentials{}
					})

					It("Succeeds", func() {
						err := job.Perform()
						Expect(err).To(BeNil())
					})
				})
			})
		})
	})

	Describe("removeContainerJob", func() {
		const (
			containerID = "ca55e77"
		)

		JustBeforeEach(func() {
			job = NewRemoveContainerJob(containerID, containerStore)
		})

		Describe("ID", func() {
			It("Returns a unique identifier for the job", func() {
				Expect(job.ID()).To(Equal("docker/remove-container/ca55e77"))
			})
		})

		Describe("Perform", func() {
			Context("When the container is not in the store", func() {
				It("Does nothing", func() {
					err := job.Perform()
					Expect(err).To(BeNil())
				})
			})

			Context("When the container is in the store", func() {
				const (
					iamRole = "an-iam-role"
					ip      = "172.0.0.7"
				)

				JustBeforeEach(func() {
					err := dockerClient.AddContainer(&dockerLib.Container{
						ID: containerID,
						Config: &dockerLib.Config{
							Labels: map[string]string{
								"com.swipely.iam-docker.iam-profile": iamRole,
							},
						},
						NetworkSettings: &dockerLib.NetworkSettings{
							IPAddress: ip,
						},
					})
					Expect(err).To(BeNil())
					err = containerStore.AddContainerByID(containerID)
					Expect(err).To(BeNil())
				})

				It("Removes the container", func() {
					roles := len(containerStore.IAMRoles())
					err := job.Perform()
					Expect(err).To(BeNil())
					newRoles := len(containerStore.IAMRoles())
					Expect(roles - newRoles).To(Equal(1))
				})
			})
		})
	})

	Describe("syncContainersJob", func() {
		JustBeforeEach(func() {
			job = NewSyncContainersJob(
				dockerClient,
				containerStore,
				credentialStore,
				jobQueue,
			)
		})

		Describe("ID", func() {
			It("Returns a unique identifier for the job", func() {
				Expect(job.ID()).To(Equal("docker/sync-containers"))
			})
		})

		Describe("Perform", func() {
			Context("When the containers cannot be listed", func() {
				JustBeforeEach(func() {
					dockerClient.SetServerError(true)
				})

				It("Fails", func() {
					err := job.Perform()
					Expect(err).ToNot(BeNil())
				})
			})

			Context("When the container can be listed", func() {
				JustBeforeEach(func() {
					dockerClient.SetServerError(false)

					err := dockerClient.AddContainer(&dockerLib.Container{
						ID: "0123",
					})
					Expect(err).To(BeNil())
					err = dockerClient.AddContainer(&dockerLib.Container{
						ID: "4567",
					})
					Expect(err).To(BeNil())
				})

				It("Enqueues a new addContainerJob for each container", func() {
					err := job.Perform()
					Expect(err).To(BeNil())
					Expect(len(jobQueue.Jobs)).To(Equal(2))
					Expect(jobQueue.Jobs[0].ID()).To(Equal("docker/add-container/0123"))
					Expect(jobQueue.Jobs[1].ID()).To(Equal("docker/add-container/4567"))
				})
			})
		})
	})
})
