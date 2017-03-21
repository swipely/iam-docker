package docker_test

import (
	"github.com/aws/aws-sdk-go/service/sts"
	docker "github.com/fsouza/go-dockerclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/iam"
	"github.com/swipely/iam-docker/src/mock"
	"sync"
	"time"
)

var _ = Describe("EventHandler", func() {
	var (
		channel         chan *docker.APIEvents
		dockerClient    *mock.DockerClient
		stsClient       *mock.STSClient
		containerStore  ContainerStore
		credentialStore iam.CredentialStore
		subject         EventHandler
		waitGroup       sync.WaitGroup
	)

	BeforeEach(func() {
		channel = make(chan *docker.APIEvents)
		dockerClient = mock.NewDockerClient()
		stsClient = mock.NewSTSClient()
		containerStore = NewContainerStore(dockerClient)
		credentialStore = iam.NewCredentialStore(stsClient, 1)
		subject = NewEventHandler(1, containerStore, credentialStore)
		_ = dockerClient.AddEventListener(channel)
		waitGroup.Add(1)
		go func() {
			_ = subject.Listen(channel)
			waitGroup.Done()
		}()
	})

	Describe("Listen", func() {
		var (
			id string
			ip string
		)

		Context("When a start event is received", func() {
			Context("When the container does not have com.swipely.iam-docker.iam-profile set", func() {
				BeforeEach(func() {
					id = "CA55E77E"
					ip = "172.17.0.2"
					_ = dockerClient.AddContainer(&docker.Container{
						ID:     id,
						Config: &docker.Config{Labels: map[string]string{}},
						NetworkSettings: &docker.NetworkSettings{
							Networks: map[string]docker.ContainerNetwork{
								"bridge": docker.ContainerNetwork{
									IPAddress: ip,
								},
							},
						},
					})
				})

				It("Does not add that container to the store", func() {
					close(channel)
					waitGroup.Wait()
					_, err := containerStore.IAMRoleForID(id)
					Expect(err).ToNot(BeNil())
				})
			})

			Context("When the container has com.swipely.iam-docker.iam-profile set", func() {
				var (
					role            = "test-role"
					externalId      = "eid"
					accessKeyID     = "test-access-key-id"
					secretAccessKey = "test-secret-access-key"
					expiration      = time.Now().Add(time.Hour)
					sessionToken    = "test-session-token"
				)

				BeforeEach(func() {
					id = "DEADBEEF"
					ip = "172.17.0.3"
					stsClient.AssumableRoles[role] = &sts.Credentials{
						AccessKeyId:     &accessKeyID,
						SecretAccessKey: &secretAccessKey,
						Expiration:      &expiration,
						SessionToken:    &sessionToken,
					}
					_ = dockerClient.AddContainer(&docker.Container{
						ID:     id,
						Config: &docker.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role, "com.swipely.iam-docker.iam-externalid": externalId}},
						NetworkSettings: &docker.NetworkSettings{
							Networks: map[string]docker.ContainerNetwork{
								"bridge": docker.ContainerNetwork{
									IPAddress: ip,
								},
							},
						},
					})
				})

				It("Attempts to assume that role", func() {
					close(channel)
					waitGroup.Wait()
					r, err := containerStore.IAMRoleForID(id)
					Expect(r.ExternalId).To(Equal("eid"))
					Expect(err).To(BeNil())
				})
			})
		})

		Context("When a die event is received", func() {
			Context("When the container is not in the store", func() {
				BeforeEach(func() {
					id = "00000000"
					ip = "172.17.0.4"
					_ = dockerClient.AddContainer(&docker.Container{
						ID:              id,
						Config:          &docker.Config{Labels: map[string]string{}},
						NetworkSettings: &docker.NetworkSettings{IPAddress: ip},
					})
				})

				It("Does nothing", func() {
					_, err := containerStore.IAMRoleForID(id)
					Expect(err).ToNot(BeNil())
					_ = dockerClient.RemoveContainer(id)
					close(channel)
					waitGroup.Wait()
					_, err = containerStore.IAMRoleForID(id)
					Expect(err).ToNot(BeNil())
				})
			})

			Context("When the container is in the store", func() {
				var (
					role            = "test-role"
					externalId      = ""
					accessKeyID     = "test-access-key-id"
					secretAccessKey = "test-secret-access-key"
					expiration      = time.Now().Add(time.Hour)
					sessionToken    = "test-session-token"
				)

				BeforeEach(func() {
					id = "11111111"
					ip = "172.17.0.5"
					stsClient.AssumableRoles[role] = &sts.Credentials{
						AccessKeyId:     &accessKeyID,
						SecretAccessKey: &secretAccessKey,
						Expiration:      &expiration,
						SessionToken:    &sessionToken,
					}
					_ = dockerClient.AddContainer(&docker.Container{
						ID:              id,
						Config:          &docker.Config{Labels: map[string]string{"com.swipely.iam-docker.iam-profile": role, "com.swipely.iam-docker.iam-externalid": externalId}},
						NetworkSettings: &docker.NetworkSettings{IPAddress: ip},
					})
					_ = dockerClient.RemoveContainer(id)
				})

				It("Removes the container", func() {
					close(channel)
					waitGroup.Wait()
					_, err := containerStore.IAMRoleForID(id)
					Expect(err).ToNot(BeNil())
					creds, err := credentialStore.CredentialsForRole(role, externalId)
					Expect(err).To(BeNil())
					Expect(*creds.AccessKeyId).To(Equal(accessKeyID))
				})
			})
		})
	})
})
