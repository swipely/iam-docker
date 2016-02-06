package iam

import (
	"fmt"
	sts "github.com/aws/aws-sdk-go/service/sts"
	"sync"
	"time"
)

const (
	refreshGracePeriod  = time.Minute * 2
	realTimeGracePeriod = time.Second * 10
)

type credentialStore struct {
	client STSClient
	creds  map[string]*sts.Credentials
	mutex  sync.RWMutex
}

// NewCredentialStore accepts an STSClient and creates a new cache for assumed
// IAM credentials.
func NewCredentialStore(client STSClient) CredentialStore {
	return &credentialStore{
		client: client,
		creds:  make(map[string]*sts.Credentials),
	}
}

func (store *credentialStore) CredentialsForRole(arn string) (*sts.Credentials, error) {
	return store.refreshCredential(arn, realTimeGracePeriod)
}

func (store *credentialStore) RefreshCredentials() {
	store.mutex.RLock()
	arns := make([]string, len(store.creds))
	count := 0
	for arn := range store.creds {
		arns[count] = arn
		count++
	}
	store.mutex.RUnlock()

	for _, arn := range arns {
		_, _ = store.refreshCredential(arn, refreshGracePeriod)
	}
}

func (store *credentialStore) refreshCredential(arn string, gracePeriod time.Duration) (*sts.Credentials, error) {
	store.mutex.RLock()
	creds, hasKey := store.creds[arn]
	store.mutex.RUnlock()

	if hasKey && time.Now().Add(gracePeriod).Before(*creds.Expiration) {
		return creds, nil
	}

	output, err := store.client.AssumeRole(&sts.AssumeRoleInput{RoleArn: &arn})

	if err != nil {
		return nil, err
	} else if output.Credentials == nil {
		return nil, fmt.Errorf("No credentials returned for: %s", arn)
	}

	store.mutex.Lock()
	store.creds[arn] = output.Credentials
	store.mutex.Unlock()

	return output.Credentials, nil
}
