package docker

import (
	"fmt"
	"sync"
)

// NewContainerStore creates an empty container store.
func NewContainerStore() ContainerStore {
	return &containerStore{
		containerNamesByIP:    make(map[string]string),
		configByContainerName: make(map[string]containerConfig),
	}
}

func (store *containerStore) AddContainer(name string, ip string, iamRole string) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.configByContainerName[name] = containerConfig{ip: ip, iamRole: iamRole}
	store.containerNamesByIP[ip] = name
}

func (store *containerStore) RemoveContainer(name string) {
	store.mutex.Lock()
	defer store.mutex.Unlock()

	config, hasKey := store.configByContainerName[name]
	if hasKey {
		delete(store.containerNamesByIP, config.ip)
		delete(store.configByContainerName, name)
	}

}

func (store *containerStore) IAMRoleForIP(ip string) (string, error) {
	store.mutex.RLock()
	defer store.mutex.RUnlock()

	name, hasKey := store.containerNamesByIP[ip]
	if !hasKey {
		return "", fmt.Errorf("Unable to find container for IP: %s", ip)
	}

	config, hasKey := store.configByContainerName[name]
	if !hasKey {
		return "", fmt.Errorf("Unable to find config for container: %s", name)
	}

	return config.iamRole, nil
}

type containerConfig struct {
	ip      string
	iamRole string
}

type containerStore struct {
	mutex                 sync.RWMutex
	containerNamesByIP    map[string]string
	configByContainerName map[string]containerConfig
}
