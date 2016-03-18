package docker

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	dockerClient "github.com/fsouza/go-dockerclient"
	"sync"
	"time"
)

const (
	iamLabel             = "IAM_PROFILE"
	retrySleepBase       = time.Second
	retrySleepMultiplier = 2
	maxRetries           = 3
)

var (
	runningContainersOpts = dockerClient.ListContainersOptions{
		All:  false,
		Size: false,
	}
)

// NewContainerStore creates an empty container store.
func NewContainerStore(client RawClient) ContainerStore {
	return &containerStore{
		containerIDsByIP:    make(map[string]string),
		configByContainerID: make(map[string]containerConfig),
		client:              client,
	}
}

func (store *containerStore) AddContainerByID(id string) error {
	logger := log.WithFields(logrus.Fields{"id": id})
	logger.Debug("Attempting to add container")
	config, err := store.findConfigForID(id)
	if err != nil {
		return err
	}

	logger.WithFields(logrus.Fields{
		"ip":   config.ip,
		"role": config.iamRole,
	}).Debug("Adding new container")
	store.mutex.Lock()
	store.containerIDsByIP[config.ip] = config.id
	store.configByContainerID[config.id] = *config
	store.mutex.Unlock()

	return nil
}

func (store *containerStore) IAMRoles() []string {
	log.Debug("Fetching unique IAM Roles in the store")

	store.mutex.RLock()
	iamSet := make(map[string]bool, len(store.configByContainerID))
	for _, config := range store.configByContainerID {
		iamSet[config.iamRole] = true
	}
	store.mutex.RUnlock()

	iamRoles := make([]string, len(iamSet))
	count := 0
	for role := range iamSet {
		iamRoles[count] = role
		count++
	}

	return iamRoles
}

func (store *containerStore) IAMRoleForID(id string) (string, error) {
	log.WithField("id", id).Debug("Looking up IAM role")

	store.mutex.RLock()
	defer store.mutex.RUnlock()

	config, hasKey := store.configByContainerID[id]
	if !hasKey {
		return "", fmt.Errorf("Unable to find config for container: %s", id)
	}

	return config.iamRole, nil
}

func (store *containerStore) IAMRoleForIP(ip string) (string, error) {
	log.WithField("ip", ip).Debug("Looking up IAM role")

	store.mutex.RLock()
	defer store.mutex.RUnlock()

	id, hasKey := store.containerIDsByIP[ip]
	if !hasKey {
		return "", fmt.Errorf("Unable to find container for IP: %s", ip)
	}

	config, hasKey := store.configByContainerID[id]
	if !hasKey {
		return "", fmt.Errorf("Unable to find config for container: %s", id)
	}

	return config.iamRole, nil
}

func (store *containerStore) RemoveContainer(id string) {
	store.mutex.RLock()
	config, hasKey := store.configByContainerID[id]
	store.mutex.RUnlock()

	if hasKey {
		log.WithField("id", id).Debug("Removing container")
		store.mutex.Lock()
		delete(store.containerIDsByIP, config.ip)
		delete(store.configByContainerID, id)
		store.mutex.Unlock()
	}

}

func (store *containerStore) SyncRunningContainers() error {
	log.Info("Syncing the running containers")

	apiContainers, err := store.listContainers()
	if err != nil {
		return err
	}

	store.mutex.Lock()
	defer store.mutex.Unlock()

	count := len(apiContainers)
	store.containerIDsByIP = make(map[string]string, count)
	store.configByContainerID = make(map[string]containerConfig, count)

	for _, container := range apiContainers {
		config, err := store.findConfigForID(container.ID)
		if err == nil {
			log.WithFields(logrus.Fields{
				"id":   config.id,
				"ip":   config.ip,
				"role": config.iamRole,
			}).Debug("Adding new container")
			store.containerIDsByIP[config.ip] = config.id
			store.configByContainerID[config.id] = *config
		}
	}

	log.Info("Done syncing the running containers, ", len(store.configByContainerID), " now in the store")

	return nil
}

func (store *containerStore) findConfigForID(id string) (*containerConfig, error) {
	container, err := store.inspectContainer(id)
	if err != nil {
		return nil, err
	} else if container == nil {
		return nil, fmt.Errorf("Cannot inspect container: %s", id)
	} else if container.Config == nil {
		return nil, fmt.Errorf("Container has no config: %s", id)
	} else if container.NetworkSettings == nil {
		return nil, fmt.Errorf("Container has no network settings: %s", id)
	}

	iamRole, hasKey := container.Config.Labels[iamLabel]
	if !hasKey {
		return nil, fmt.Errorf("Unable to find label named IAM_PROFILE for container: %s", id)
	}
	ip := container.NetworkSettings.IPAddress
	config := &containerConfig{
		id:      id,
		ip:      ip,
		iamRole: iamRole,
	}

	return config, nil
}

func (store *containerStore) listContainers() ([]dockerClient.APIContainers, error) {
	log.Debug("Listing containers")
	var containers []dockerClient.APIContainers
	err := withRetries(func() error {
		var e error
		containers, e = store.client.ListContainers(runningContainersOpts)
		return e
	})
	return containers, err
}

func (store *containerStore) inspectContainer(id string) (*dockerClient.Container, error) {
	log.WithField("id", id).Debug("Inspecting container")
	var container *dockerClient.Container
	err := withRetries(func() error {
		var e error
		container, e = store.client.InspectContainer(id)
		return e
	})
	return container, err
}

func withRetries(lambda func() error) error {
	var err error
	sleepTime := retrySleepBase

	for attempt := 0; attempt < maxRetries; attempt++ {
		err = lambda()
		if err == nil {
			break
		}
		time.Sleep(sleepTime)
		sleepTime *= retrySleepMultiplier
	}

	return err
}

type containerConfig struct {
	id      string
	ip      string
	iamRole string
}

type containerStore struct {
	mutex               sync.RWMutex
	containerIDsByIP    map[string]string
	configByContainerID map[string]containerConfig
	client              RawClient
}
