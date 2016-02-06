package iam

import (
	"github.com/Sirupsen/logrus"
	sts "github.com/aws/aws-sdk-go/service/sts"
)

var (
	log = logrus.WithFields(logrus.Fields{"package": "iam"})
)

// STSClient specifies the subset of STS API calls used by the CredentialStore.
type STSClient interface {
	AssumeRole(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)
}

// CredentialStore caches IAM credentials and can refresh those which are going
// stale.
type CredentialStore interface {
	// Lookup the credentials for the given ARN.
	CredentialsForRole(arn string) (*sts.Credentials, error)
	// Refresh all the credentials that are expired or are about to expire.
	RefreshCredentials()
}
