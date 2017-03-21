package iam

import (
	"github.com/aws/aws-sdk-go/service/sts"
)

// STSClient specifies the subset of STS API calls used by the CredentialStore.
type STSClient interface {
	AssumeRole(*sts.AssumeRoleInput) (*sts.AssumeRoleOutput, error)
}

// CredentialStore caches IAM credentials and can refresh those which are going
// stale.
type CredentialStore interface {
	// Lookup the credentials for the given ARN.
	CredentialsForRole(arn, externalId string) (*sts.Credentials, error)
	// Refresh all the credentials that are expired or are about to expire.
	RefreshCredentials()
}
