package iam_test

import (
	"github.com/aws/aws-sdk-go/service/sts"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/swipely/iam-docker/src/iam"
	"github.com/swipely/iam-docker/src/mock"
	"time"
)

var _ = Describe("CredentialStore", func() {
	var (
		client  *mock.STSClient
		subject CredentialStore
	)

	BeforeEach(func() {
		client = mock.NewSTSClient()
		subject = NewCredentialStore(client, 1)
	})

	Describe("CredentialsForRole", func() {
		const (
			role       = "arn:aws:iam::012345678901:role/test"
			externalId = ""
		)

		Context("When the credentials have not been assumed", func() {
			Context("When the credentials cannot be assumed", func() {
				It("Returns an error", func() {
					creds, err := subject.CredentialsForRole(role, externalId)
					Expect(creds).To(BeNil())
					Expect(err).ToNot(BeNil())
				})
			})

			Context("When the credentials can be assumed", func() {
				var (
					accessKeyID     = "fakeaccesskeyid"
					secretAccessKey = "fakesecretaccesskey"
					expiration      = time.Now().Add(time.Hour)
					sessionToken    = "fakesessiontoken"
				)

				BeforeEach(func() {
					client.AssumableRoles[role] = &sts.Credentials{
						AccessKeyId:     &accessKeyID,
						SecretAccessKey: &secretAccessKey,
						Expiration:      &expiration,
						SessionToken:    &sessionToken,
					}
				})

				It("Returns the credentials", func() {
					creds, err := subject.CredentialsForRole(role, externalId)
					Expect(creds).ToNot(BeNil())
					Expect(err).To(BeNil())
					Expect(*creds.AccessKeyId).To(Equal(accessKeyID))
					Expect(*creds.SecretAccessKey).To(Equal(secretAccessKey))
					Expect(*creds.Expiration).To(Equal(expiration))
					Expect(*creds.SessionToken).To(Equal(sessionToken))
				})
			})
		})

		Context("When the credentials have been assumed", func() {
			var (
				accessKeyID     = "fakeaccesskeyid"
				expiration      = time.Now().Add(time.Hour)
				secretAccessKey = "fakesecretaccesskey"
				sessionToken    = "fakesessiontoken"
				creds           = &sts.Credentials{
					AccessKeyId:     &accessKeyID,
					Expiration:      &expiration,
					SecretAccessKey: &secretAccessKey,
					SessionToken:    &sessionToken,
				}
			)

			BeforeEach(func() {
				client.AssumableRoles[role] = creds
				_, _ = subject.CredentialsForRole(role, externalId)
			})

			Context("But they are about to go stale", func() {
				var (
					newExpiration time.Time
				)

				JustBeforeEach(func() {
					expiration = time.Now().Add(5 * time.Second)
					newExpiration = time.Now().Add(time.Hour)
					creds.Expiration = &expiration
					client.AssumableRoles[role] = &sts.Credentials{
						AccessKeyId:     &accessKeyID,
						Expiration:      &newExpiration,
						SecretAccessKey: &secretAccessKey,
						SessionToken:    &sessionToken,
					}
				})

				It("Refreshes them", func() {
					creds, err := subject.CredentialsForRole(role, externalId)
					Expect(creds).ToNot(BeNil())
					Expect(err).To(BeNil())
					Expect(*creds.AccessKeyId).To(Equal(accessKeyID))
					Expect(*creds.SecretAccessKey).To(Equal(secretAccessKey))
					Expect(*creds.Expiration).To(Equal(newExpiration))
					Expect(*creds.SessionToken).To(Equal(sessionToken))
				})
			})

			Context("And they are fresh", func() {
				JustBeforeEach(func() {
					expiration = time.Now().Add(5 * time.Hour)
					creds.Expiration = &expiration
				})

				It("Returns the credentials", func() {
					creds, err := subject.CredentialsForRole(role, externalId)
					Expect(creds).ToNot(BeNil())
					Expect(err).To(BeNil())
					Expect(*creds.AccessKeyId).To(Equal(accessKeyID))
					Expect(*creds.SecretAccessKey).To(Equal(secretAccessKey))
					Expect(*creds.Expiration).To(Equal(expiration))
					Expect(*creds.SessionToken).To(Equal(sessionToken))
				})
			})
		})
	})

	Describe("RefreshCredentials", func() {
		var (
			role            = "arn:aws:iam::012345678901:role/test"
			externalId      = ""
			accessKeyID     = "fakeaccesskeyid"
			oldExpiration   = time.Now()
			newExpiration   = time.Now().Add(time.Hour)
			secretAccessKey = "fakesecretaccesskey"
			sessionToken    = "fakesessiontoken"
			creds           = &sts.Credentials{
				AccessKeyId:     &accessKeyID,
				Expiration:      &oldExpiration,
				SecretAccessKey: &secretAccessKey,
				SessionToken:    &sessionToken,
			}
			newCreds = &sts.Credentials{
				AccessKeyId:     &accessKeyID,
				Expiration:      &newExpiration,
				SecretAccessKey: &secretAccessKey,
				SessionToken:    &sessionToken,
			}
		)

		JustBeforeEach(func() {
			client.AssumableRoles[role] = creds
			_, _ = subject.CredentialsForRole(role, externalId)
			client.AssumableRoles[role] = newCreds
		})

		It("Refreshes each credential in the store", func() {
			found, err := subject.CredentialsForRole(role, externalId)
			Expect(creds).ToNot(BeNil())
			Expect(err).To(BeNil())
			Expect(*found.AccessKeyId).To(Equal(accessKeyID))
			Expect(*found.SecretAccessKey).To(Equal(secretAccessKey))
			Expect(*found.Expiration).To(Equal(newExpiration))
			Expect(*found.SessionToken).To(Equal(sessionToken))
		})
	})
})
