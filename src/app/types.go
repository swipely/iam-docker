package app

import (
	"github.com/Sirupsen/logrus"
	"github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/iam"
	"net/url"
	"time"
)

var (
	log = logrus.WithFields(logrus.Fields{"package": "app"})
)

// App holds the state of the application.
type App struct {
	Config       *Config
	DockerClient docker.RawClient
	STSClient    iam.STSClient
	ErrorChan    chan<- error
}

// Config holds application configuration
type Config struct {
	ListenAddr              string
	MetaDataUpstream        *url.URL
	EventHandlers           int
	ReadTimeout             time.Duration
	WriteTimeout            time.Duration
	DockerSyncPeriod        time.Duration
	CredentialRefreshPeriod time.Duration
}
