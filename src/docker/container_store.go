package docker

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	dockerClient "github.com/fsouza/go-dockerclient"
	"sync"
	"time"
)

const (
	iamLabel                         = "com.swipely.iam-docker.iam-profile"
	iamExternalIdLabel               = "com.swipely.iam-docker.iam-externalid"
	iamEnvironmentVariable           = "IAM_ROLE"
	iamExternalIdEnvironmentVariable = "IAM_ROLE_EXTERNALID"
	retrySleepBase                   = time.Second
	retrySleepMultiplier             = 2
	maxRetries                       = 3
)

var (
	runningContainersOpts = dockerClient.ListContainersOptions{
		All:  false,
		Size: false,
	}
)

type ComplexRole struct {
	Arn        string
	ExternalId string
}

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

	for _, ip := range config.ips {
		logger.WithFields(logrus.Fields{
			"ip":   ip,
			"role": config.iamRole,
		}).Debug("Adding new container")
	}

	store.mutex.Lock()
	for _, ip := range config.ips {
		store.containerIDsByIP[ip] = config.id
	}
	store.configByContainerID[config.id] = *config
	store.mutex.Unlock()

	return nil
}

func (store *containerStore) IAMRoles() []ComplexRole {
	log.Debug("Fetching unique IAM Roles in the store")

	store.mutex.RLock()
	iamSet := make(map[string]bool, len(store.configByContainerID))
	externalId := make(map[string]string, len(store.configByContainerID))
	for _, config := range store.configByContainerID {
		iamSet[config.iamRole] = true
		externalId[config.iamRole] = config.externalId
	}
	store.mutex.RUnlock()

	iamRoles := make([]ComplexRole, len(iamSet))
	count := 0
	for role := range iamSet {
		r := ComplexRole{
			Arn:        role,
			ExternalId: externalId[role],
		}
		iamRoles[count] = r
		count++
	}

	return iamRoles
}

func (store *containerStore) IAMRoleForID(id string) (ComplexRole, error) {
	log.WithField("id", id).Debug("Looking up IAM role")

	store.mutex.RLock()
	defer store.mutex.RUnlock()

	config, hasKey := store.configByContainerID[id]
	if !hasKey {
		return ComplexRole{}, fmt.Errorf("Unable to find config for container: %s", id)
	}

	iamRole := ComplexRole{
		Arn:        config.iamRole,
		ExternalId: config.externalId,
	}
	return iamRole, nil
}

func (store *containerStore) IAMRoleForIP(ip string) (ComplexRole, error) {
	log.WithField("ip", ip).Debug("Looking up IAM role")

	store.mutex.RLock()
	defer store.mutex.RUnlock()

	id, hasKey := store.containerIDsByIP[ip]
	if !hasKey {
		return ComplexRole{}, fmt.Errorf("Unable to find container for IP: %s", ip)
	}

	config, hasKey := store.configByContainerID[id]
	if !hasKey {
		return ComplexRole{}, fmt.Errorf("Unable to find config for container: %s", id)
	}

	iamRole := ComplexRole{
		Arn:        config.iamRole,
		ExternalId: config.externalId,
	}
	return iamRole, nil
}

func (store *containerStore) RemoveContainer(id string) {
	store.mutex.RLock()
	config, hasKey := store.configByContainerID[id]
	store.mutex.RUnlock()

	if hasKey {
		log.WithField("id", id).Debug("Removing container")
		store.mutex.Lock()
		for _, ip := range config.ips {
			delete(store.containerIDsByIP, ip)
		}
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
			for _, ip := range config.ips {
				log.WithFields(logrus.Fields{
					"id":   config.id,
					"ip":   ip,
					"role": config.iamRole,
				}).Debug("Adding new container")
				store.containerIDsByIP[ip] = config.id
			}
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

	externalId, _ := container.Config.Labels[iamExternalIdLabel]
	iamRole, hasLabel := container.Config.Labels[iamLabel]
	if !hasLabel {
		env := dockerClient.Env(container.Config.Env)
		envRole := env.Get(iamEnvironmentVariable)
		envExternalId := env.Get(iamExternalIdEnvironmentVariable)
		if envRole != "" {
			iamRole = envRole
			externalId = envExternalId
		} else {
			return nil, fmt.Errorf("Unable to find label named '%s' or environment variable '%s' for container: %s", iamLabel, iamEnvironmentVariable, id)
		}
	}

	ips := make([]string, 0, 2)
	for _, network := range container.NetworkSettings.Networks {
		ip := network.IPAddress
		if ip != "" {
			ips = append(ips, ip)
		}
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("Unable to find IP address for container: %s", id)
	}

	config := &containerConfig{
		id:         id,
		ips:        ips,
		iamRole:    iamRole,
		externalId: externalId,
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
	id         string
	ips        []string
	iamRole    string
	externalId string
}

type containerStore struct {
	mutex               sync.RWMutex
	containerIDsByIP    map[string]string
	configByContainerID map[string]containerConfig
	client              RawClient
}
