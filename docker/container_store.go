package docker

import (
	"fmt"
	dockerClient "github.com/fsouza/go-dockerclient"
	"strings"
	"sync"
	"time"
)

const (
	iamPrefix            = "IAM_PROFILE="
	retrySleepBase       = time.Second
	retrySleepMultiplier = 2
	maxRetries           = 3
)

var (
	runningContainersOpts = dockerClient.ListContainersOptions{
		Filters: map[string][]string{
			"status=": []string{"running"},
		},
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
	config, err := store.findConfigForID(id)
	if err != nil {
		return err
	}

	store.mutex.Lock()
	store.containerIDsByIP[config.ip] = config.id
	store.configByContainerID[config.id] = *config
	store.mutex.Unlock()

	return nil

}

func (store *containerStore) IAMRoleForIP(ip string) (string, error) {
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
		store.mutex.Lock()
		delete(store.containerIDsByIP, config.ip)
		delete(store.configByContainerID, id)
		store.mutex.Unlock()
	}

}

func (store *containerStore) SyncRunningContainers() error {
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
			store.containerIDsByIP[config.ip] = config.id
			store.configByContainerID[config.id] = *config
		}
	}

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

	iamRole, err := findIAMRole(container.Config.Env)
	if err != nil {
		return nil, err
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
	var containers []dockerClient.APIContainers
	err := withRetries(func() error {
		var e error
		containers, e = store.client.ListContainers(runningContainersOpts)
		return e
	})
	return containers, err
}

func (store *containerStore) inspectContainer(id string) (*dockerClient.Container, error) {
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

func findIAMRole(env []string) (string, error) {
	if env != nil {
		for _, element := range env {
			if strings.HasPrefix(element, iamPrefix) {
				return strings.TrimPrefix(element, iamPrefix), nil
			}
		}
	}

	return "", fmt.Errorf("Unable to find environment variable with prefix: %s", iamPrefix)
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
