package app

import (
	"github.com/Sirupsen/logrus"
	dockerLib "github.com/fsouza/go-dockerclient"
	"github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/http"
	"github.com/swipely/iam-docker/src/iam"
	"github.com/valyala/fasthttp"
	"hash/fnv"
	"net/http/httputil"
	"os"
	"time"
)

var (
	log = logrus.WithField("prefix", "app")
)

// New creates a new application with the given config.
func New(config *Config, dockerClient docker.RawClient, stsClient iam.STSClient) *App {
	return &App{
		Config:       config,
		DockerClient: dockerClient,
		STSClient:    stsClient,
	}
}

// Run starts the application asynchronously.
func (app *App) Run() error {
	log.Info("Running the app")

	errorChan := make(chan error)
	containerStore := docker.NewContainerStore(app.DockerClient)
	credentialStore := iam.NewCredentialStore(app.STSClient, app.randomSeed())
	eventHandler := docker.NewEventHandler(app.Config.EventHandlers, containerStore, credentialStore)
	proxy := httputil.NewSingleHostReverseProxy(app.Config.MetaDataUpstream)
	handler := http.NewIAMHandler(proxy, containerStore, credentialStore, app.Config.DisableUpstream)

	go app.containerSyncWorker(containerStore, credentialStore)
	go app.refreshCredentialWorker(credentialStore)
	go app.httpWorker(handler, errorChan)
	go app.eventWorker(eventHandler, errorChan)

	return <-errorChan
}

func (app *App) containerSyncWorker(containerStore docker.ContainerStore, credentialStore iam.CredentialStore) {
	wlog := log.WithFields(logrus.Fields{"worker": "sync-containers"})
	wlog.Info("Starting")

	go app.syncRunningContainers(containerStore, credentialStore, wlog)

	// Don't sync every minute since we're already listening to Docker events.
	// This is the default.
	if app.Config.DockerSyncPeriod == (0 * time.Second) {
		return
	}

	timer := time.Tick(app.Config.DockerSyncPeriod)
	for range timer {
		go app.syncRunningContainers(containerStore, credentialStore, wlog)
	}
}

func (app *App) refreshCredentialWorker(credentialStore iam.CredentialStore) {
	timer := time.Tick(app.Config.CredentialRefreshPeriod)
	wlog := log.WithFields(logrus.Fields{"worker": "refresh-credentials"})
	wlog.Info("Starting")

	for range timer {
		wlog.Debug("Refreshing credentials")
		go credentialStore.RefreshCredentials()
	}
}

func (app *App) httpWorker(handler fasthttp.RequestHandler, errorChan chan error) {
	wlog := log.WithFields(logrus.Fields{"worker": "http"})
	wlog.Info("Starting")
	server := fasthttp.Server{
		Handler:      handler,
		Name:         "iam-docker",
		ReadTimeout:  app.Config.ReadTimeout,
		WriteTimeout: app.Config.WriteTimeout,
		Logger:       wlog.Logger,
		DisableHeaderNamesNormalizing: true,
	}
	err := server.ListenAndServe(app.Config.ListenAddr)
	wlog.WithFields(logrus.Fields{
		"error": err.Error(),
	}).Error("Failed to serve HTTP")
	errorChan <- err
}

func (app *App) eventWorker(eventHandler docker.EventHandler, errorChan chan error) {
	wlog := log.WithFields(logrus.Fields{"worker": "event-handler"})
	wlog.Info("Starting")
	for {
		wlog.Debug("Listening for Docker events")
		events := make(chan *dockerLib.APIEvents, app.Config.EventHandlers)
		err := app.DockerClient.AddEventListener(events)
		if err != nil {
			wlog.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Error("Failed to add Docker event listener")
			errorChan <- err
			return
		}
		err = eventHandler.Listen(events)
		if err != nil {
			wlog.WithFields(logrus.Fields{
				"error": err.Error(),
			}).Warn("Exited")
		} else {
			wlog.Warn("Exited with no error")
		}
	}
}

func (app *App) syncRunningContainers(containerStore docker.ContainerStore, credentialStore iam.CredentialStore, logger *logrus.Entry) {
	logger.Info("Syncing containers")
	err := containerStore.SyncRunningContainers()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("Failed syncing running containers")
	}
	for _, role := range containerStore.IAMRoles() {
		_, err := credentialStore.CredentialsForRole(role.Arn, role.ExternalId)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"arn":   role,
				"error": err.Error(),
			}).Warn("Unable to fetch credential")
		} else {
			logger.WithFields(logrus.Fields{
				"arn": role,
			}).Info("Successfully fetched credential")
		}
	}
}

func (app *App) randomSeed() int64 {
	nano := time.Now().UnixNano()
	hostname, err := os.Hostname()
	if err != nil {
		log.WithField("error", err).Warn("Unable to fetch Hostname")
		return nano
	}
	hash := fnv.New64a()
	hash.Write([]byte(hostname))
	return nano ^ int64(hash.Sum64())
}
