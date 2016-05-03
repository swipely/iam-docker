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
	refreshCredentialGracePeriod = 30 * time.Minute
)

// NewCredentialStore accepts an STSClient and creates a new cache for assumed
// IAM credentials.
func NewCredentialStore(client STSClient, seed int64) CredentialStore {
	return &credentialStore{
		client: client,
		creds:  make(map[string]*sts.Credentials),
		rng:    rand.New(rand.NewSource(seed)),
		logger: logrus.WithField("prefix", "iam/credential-store"),
	}
}

func (store *credentialStore) CredentialsForRole(arn string) (*sts.Credentials, error) {
	store.credMutex.RLock()
	creds, hasKey := store.creds[arn]
	store.credMutex.RUnlock()

	if !hasKey {
		return nil, fmt.Errorf("No credentials for role: %s", arn)
	}
	return creds, nil
}

func (store *credentialStore) AvailableARNs() []string {
	store.credMutex.RLock()
	defer store.credMutex.RUnlock()
	arns := make([]string, len(store.creds))
	count := 0
	for arn := range store.creds {
		arns[count] = arn
		count++
	}
	return arns
}

func (store *credentialStore) RefreshCredentials() {
	store.logger.Info("Refreshing all IAM credentials")
	for _, arn := range store.AvailableARNs() {
		err := store.RefreshCredentialIfStale(arn)
		if err != nil {
			store.logger.WithFields(logrus.Fields{
				"role":  arn,
				"error": err.Error(),
			}).Warn("Unable to refresh credential")
		}
	}
	store.logger.Info("Done refreshing all IAM credentials")
}

func (store *credentialStore) RefreshCredentialIfStale(arn string) error {
	logger := store.logger.WithField("arn", arn)

	store.credMutex.RLock()
	cred, hasKey := store.creds[arn]
	store.credMutex.RUnlock()

	if hasKey {
		if time.Now().Add(refreshCredentialGracePeriod).Before(*cred.Expiration) {
			logger.Debug("Credential is fresh, refusing to refresh")
			return nil
		}
		logger.Debug("Credential is stale, refreshing")
	} else {
		logger.Debug("Credential is not in the store, fetching")
	}

	logger.Debug("Refreshing credential")
	duration := int64(3600)
	sessionName := store.generateSessionName()
	output, err := store.client.AssumeRole(&sts.AssumeRoleInput{
		RoleArn:         &arn,
		DurationSeconds: &duration,
		RoleSessionName: &sessionName,
	})
	if err != nil {
		return err
	} else if output.Credentials == nil {
		return fmt.Errorf("No credentials returned for: %s", arn)
	}

	store.credMutex.Lock()
	store.creds[arn] = output.Credentials
	store.credMutex.Unlock()

	logger.Info("Credential successfully fetched")

	return nil
}

func (store *credentialStore) generateSessionName() string {
	ary := [16]byte{}
	idx := 0
	for idx < 16 {
		store.rngMutex.Lock()
		int := store.rng.Int63()
		store.rngMutex.Unlock()
		for (int > 0) && (idx < 16) {
			ary[idx] = byte((int % 26) + 65)
			int /= 26
			idx++
		}
	}
	return string(ary[:])
}

type credentialStore struct {
	client    STSClient
	creds     map[string]*sts.Credentials
	rng       *rand.Rand
	rngMutex  sync.Mutex
	credMutex sync.RWMutex
	logger    *logrus.Entry
}
