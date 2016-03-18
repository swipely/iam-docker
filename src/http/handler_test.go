package http_test

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/service/sts"
	dockerlib "github.com/fsouza/go-dockerclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/swipely/iam-docker/src/docker"
	. "github.com/swipely/iam-docker/src/http"
	"github.com/swipely/iam-docker/src/iam"
	"github.com/swipely/iam-docker/src/mock"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"time"
)

var _ = Describe("Handler", func() {
	var (
		subject         http.Handler
		upstream        http.Handler
		containerStore  docker.ContainerStore
		credentialStore iam.CredentialStore
		stsClient       *mock.STSClient
		dockerClient    *mock.DockerClient
		request         *http.Request
		url             *neturl.URL
		path            string
		writer          *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		writer = httptest.NewRecorder()
		upstream = mock.NewHandler(func(writer http.ResponseWriter, request *http.Request) {
			_, err := writer.Write([]byte(request.URL.Path))
			Expect(err).To(BeNil())
		})
		stsClient = mock.NewSTSClient()
		dockerClient = mock.NewDockerClient()
		containerStore = docker.NewContainerStore(dockerClient)
		credentialStore = iam.NewCredentialStore(stsClient)
		subject = NewIAMHandler(upstream, containerStore, credentialStore)
	})

	Describe("ServeHTTP", func() {
		Context("When the request is for the IAM path", func() {
			const (
				ip = "172.17.81.2"
			)

			JustBeforeEach(func() {
				var err error
				path = "/latest/meta-data/iam/security-credentials/test-iam-role"
				url, err = neturl.ParseRequestURI(path)
				Expect(err).To(BeNil())
				request = &http.Request{
					Method:     "GET",
					RemoteAddr: ip,
					URL:        url,
				}
			})

			Context("When the ContainerStore cannot find that container", func() {
				It("Returns 'Not Found'", func() {
					subject.ServeHTTP(writer, request)
					Expect(writer.Code).To(Equal(404))
				})
			})

			Context("When the ContainerStore can find that container", func() {
				const (
					containerID = "DEADBEEF"
					iamRole     = "arn:aws::iam::1234123412:role/test-iam-role"
				)

				JustBeforeEach(func() {
					_ = dockerClient.AddContainer(&dockerlib.Container{
						ID: containerID,
						Config: &dockerlib.Config{
							Labels: map[string]string{"IAM_PROFILE": iamRole},
						},
						NetworkSettings: &dockerlib.NetworkSettings{
							IPAddress: ip,
						},
					})
					err := containerStore.AddContainerByID(containerID)
					Expect(err).To(BeNil())
				})

				Context("When the CredentialStore cannot find the role", func() {
					It("Returns 'Not Found'", func() {
						subject.ServeHTTP(writer, request)
						Expect(writer.Code).To(Equal(404))
					})
				})

				Context("When the CredentialStore can find that role", func() {
					var (
						accessKeyID     = "fakeaccesskeyid"
						secretAccessKey = "fakesecretaccesskey"
						expiration      = time.Now().Add(time.Hour)
						sessionToken    = "fakesessiontoken"
					)

					JustBeforeEach(func() {
						stsClient.AssumableRoles[iamRole] = &sts.Credentials{
							AccessKeyId:     &accessKeyID,
							Expiration:      &expiration,
							SecretAccessKey: &secretAccessKey,
							SessionToken:    &sessionToken,
						}
					})

					It("Returns the credentials", func() {
						var response CredentialResponse
						subject.ServeHTTP(writer, request)
						Expect(writer.Code).To(Equal(200))
						err := json.Unmarshal(writer.Body.Bytes(), &response)
						Expect(err).To(BeNil())
						Expect(response.AccessKeyID).To(Equal(accessKeyID))
						Expect(response.Code).To(Equal("Success"))
						Expect(response.Expiration.Unix()).To(Equal(expiration.Unix()))
						Expect(response.LastUpdated.Unix()).To(Equal(expiration.Add(-1 * time.Hour).Unix()))
						Expect(response.SecretAccessKey).To(Equal(secretAccessKey))
						Expect(response.Token).To(Equal(sessionToken))
						Expect(response.Type).To(Equal("AWS-HMAC"))
					})
				})
			})
		})
		Context("When the request is for the list IAM path", func() {
			const (
				ip = "172.17.81.3"
			)

			JustBeforeEach(func() {
				var err error
				path = "/latest/meta-data/iam/security-credentials/"
				url, err = neturl.ParseRequestURI(path)
				Expect(err).To(BeNil())
				request = &http.Request{
					Method:     "GET",
					RemoteAddr: ip,
					URL:        url,
				}
			})

			Context("When the ContainerStore cannot find that container", func() {
				It("Returns 'Not Found'", func() {
					subject.ServeHTTP(writer, request)
					Expect(writer.Code).To(Equal(404))
				})
			})

			Context("When the ContainerStore can find that container", func() {
				const (
					containerID = "CA55E77E"
					iamRole     = "arn:aws::iam::1234123412:role/other-iam-role"
				)

				JustBeforeEach(func() {
					_ = dockerClient.AddContainer(&dockerlib.Container{
						ID: containerID,
						Config: &dockerlib.Config{
							Labels: map[string]string{"IAM_PROFILE": iamRole},
						},
						NetworkSettings: &dockerlib.NetworkSettings{
							IPAddress: ip,
						},
					})
					err := containerStore.AddContainerByID(containerID)
					Expect(err).To(BeNil())
				})

				Context("When the CredentialStore cannot find the role", func() {
					It("Returns 'Not Found'", func() {
						subject.ServeHTTP(writer, request)
						Expect(writer.Code).To(Equal(404))
					})
				})

				Context("When the CredentialStore can find that role", func() {
					JustBeforeEach(func() {
						stsClient.AssumableRoles[iamRole] = &sts.Credentials{}
					})

					It("Returns the role name", func() {
						subject.ServeHTTP(writer, request)
						Expect(writer.Code).To(Equal(200))
						Expect(string(writer.Body.Bytes())).To(Equal("other-iam-role"))
					})
				})
			})
		})

		Context("When the request is not for the IAM path", func() {
			JustBeforeEach(func() {
				var err error
				path = "/not/an/iam/request"
				url, err = neturl.ParseRequestURI(path)
				Expect(err).To(BeNil())
				request = &http.Request{
					Method: "GET",
					URL:    url,
				}
			})

			It("Delegates to the upstream proxy", func() {
				subject.ServeHTTP(writer, request)
				Expect(writer.Code).To(Equal(200))
				Expect(writer.Body.String()).To(Equal(path))
			})
		})
	})
})
