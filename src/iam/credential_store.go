package iam

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/service/sts"
	"math/rand"
	"sync"
	"time"
)

const (
	refreshGracePeriod  = time.Minute * 2
	realTimeGracePeriod = time.Second * 10
)

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
	log.Info("Refreshing all IAM credentials")
	store.mutex.RLock()
	arns := make([]string, len(store.creds))
	count := 0
	for arn := range store.creds {
		arns[count] = arn
		count++
	}
	store.mutex.RUnlock()

	for _, arn := range arns {
		_, err := store.refreshCredential(arn, refreshGracePeriod)
		log.WithFields(logrus.Fields{
			"role":  arn,
			"error": err.Error(),
		}).Warn("Unable to refresh credential")
	}
	log.Info("Done refreshing all IAM credentials")
}

func (store *credentialStore) refreshCredential(arn string, gracePeriod time.Duration) (*sts.Credentials, error) {
	clog := log.WithFields(logrus.Fields{"arn": arn})
	clog.Info("Checking for stale credential")
	store.mutex.RLock()
	creds, hasKey := store.creds[arn]
	store.mutex.RUnlock()

	if hasKey {
		if time.Now().Add(gracePeriod).Before(*creds.Expiration) {
			clog.Info("Credential is fresh")
			return creds, nil
		}
		clog.Info("Credential is stale, refreshing")
	} else {
		clog.Info("Credential is not in the store")
	}

	duration := int64(3600)
	sessionName := generateSessionName()

	output, err := store.client.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         &arn,
		DurationSeconds: &duration,
		RoleSessionName: &sessionName,
	})

	if err != nil {
		return nil, err
	} else if output.Credentials == nil {
		return nil, fmt.Errorf("No credentials returned for: %s", arn)
	}

	clog.Info("Credential successfully refreshed")
	store.mutex.Lock()
	store.creds[arn] = output.Credentials
	store.mutex.Unlock()

	return output.Credentials, nil
}

func generateSessionName() string {
	size := 16
	ary := make([]byte, size)
	for i := range ary {
		ary[i] = byte((rand.Int() % 26) + 65)
	}
	return string(ary)
}

type credentialStore struct {
	client STSClient
	creds  map[string]*sts.Credentials
	mutex  sync.RWMutex
}
