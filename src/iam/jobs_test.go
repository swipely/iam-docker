package iam_test

import (
	"sort"

	"github.com/aws/aws-sdk-go/service/sts"

	. "github.com/swipely/iam-docker/src/iam"
	"github.com/swipely/iam-docker/src/mock"
	"github.com/swipely/iam-docker/src/queue"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("IAM Jobs", func() {
	var (
		jobQueue        *mock.JobQueue
		stsClient       *mock.STSClient
		credentialStore CredentialStore
		job             queue.Job
	)

	BeforeEach(func() {
		jobQueue = mock.NewJobQueue()
		stsClient = mock.NewSTSClient()
		credentialStore = NewCredentialStore(stsClient, 1)
	})

	Describe("refreshCredentialJob", func() {
		const (
			arn = "arn:test"
		)

		JustBeforeEach(func() {
			job = NewRefreshCredentialJob(arn, credentialStore)
		})

		Describe("ID", func() {
			It("Returns a unique identifier for the job", func() {
				Expect(job.ID()).To(Equal("iam/refresh-credential/arn:test"))
			})
		})

		Describe("Perform", func() {
			Context("When refreshing the credential fails", func() {
				It("Returns an error", func() {
					err := job.Perform()
					Expect(err).ToNot(BeNil())
					Expect(credentialStore.AvailableARNs()).ToNot(ContainElement(arn))
				})
			})

			Context("When refreshing the credential succeeds", func() {
				JustBeforeEach(func() {
					stsClient.AssumableRoles[arn] = &sts.Credentials{}
				})

				It("Returns nothing", func() {
					err := job.Perform()
					Expect(err).To(BeNil())
					Expect(credentialStore.AvailableARNs()).To(ContainElement(arn))
				})
			})
		})
	})

	Describe("refreshCredentialsJob", func() {
		JustBeforeEach(func() {
			job = NewRefreshCredentialsJob(credentialStore, jobQueue)
		})

		Describe("ID", func() {
			It("Returns a unique identifier for the job", func() {
				Expect(job.ID()).To(Equal("iam/refresh-credentials"))
			})
		})

		Describe("Perform", func() {
			var (
				arns = []string{"arn:test:1", "arn:test:2"}
			)

			JustBeforeEach(func() {
				sort.Strings(arns)
				for _, arn := range arns {
					stsClient.AssumableRoles[arn] = &sts.Credentials{}
					err := credentialStore.RefreshCredentialIfStale(arn)
					Expect(err).To(BeNil())
				}
			})

			It("Enqueues jobs to refresh credentials for each arn in the store", func() {
				err := job.Perform()
				Expect(err).To(BeNil())
				ids := make([]string, len(jobQueue.Jobs))
				for i, job := range jobQueue.Jobs {
					ids[i] = job.ID()
				}
				sort.Strings(ids)
				Expect(ids[0]).To(Equal("iam/refresh-credential/" + arns[0]))
				Expect(ids[1]).To(Equal("iam/refresh-credential/" + arns[1]))
			})
		})
	})
})
