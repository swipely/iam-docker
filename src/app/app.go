package app

import (
	"github.com/Sirupsen/logrus"
	dockerLib "github.com/fsouza/go-dockerclient"
	"github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/http"
	"github.com/swipely/iam-docker/src/iam"
	stdLog "log"
	netHTTP "net/http"
	"net/http/httputil"
	"time"
)

var (
	log = logrus.WithField("prefix", "app")
)

// New creates a new application with the given config.
func New(config *Config, dockerClient docker.RawClient, stsClient iam.STSClient, errorChan chan<- error) *App {
	return &App{
		Config:       config,
		DockerClient: dockerClient,
		STSClient:    stsClient,
		ErrorChan:    errorChan,
	}
}

// Run starts the application asynchronously.
func (app *App) Run() {
	log.Info("Running the app")

	containerStore := docker.NewContainerStore(app.DockerClient)
	credentialStore := iam.NewCredentialStore(app.STSClient)
	eventHandler := docker.NewEventHandler(app.Config.EventHandlers, containerStore, credentialStore)
	proxy := httputil.NewSingleHostReverseProxy(app.Config.MetaDataUpstream)
	handler := http.NewIAMHandler(proxy, containerStore, credentialStore)

	go app.containerSyncWorker(containerStore, credentialStore)
	go app.refreshCredentialWorker(credentialStore)
	go app.httpWorker(handler)
	go app.eventWorker(eventHandler)
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

func (app *App) httpWorker(handler netHTTP.Handler) {
	wlog := log.WithFields(logrus.Fields{"worker": "http"})
	writer := wlog.Logger.Writer()
	server := netHTTP.Server{
		Addr:           app.Config.ListenAddr,
		Handler:        handler,
		ReadTimeout:    app.Config.ReadTimeout,
		WriteTimeout:   app.Config.WriteTimeout,
		MaxHeaderBytes: 1 << 20,
		ErrorLog:       stdLog.New(writer, "", 0),
	}
	wlog.Info("Starting")
	err := server.ListenAndServe()
	wlog.WithFields(logrus.Fields{
		"error": err.Error(),
	}).Error("Failed to serve HTTP")
	_ = writer.Close()
	app.ErrorChan <- err
}

func (app *App) eventWorker(eventHandler docker.EventHandler) {
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
			app.ErrorChan <- err
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
	logger.Debug("Syncing containers")
	err := containerStore.SyncRunningContainers()
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("Failed syncing running containers")
	}
	for _, arn := range containerStore.IAMRoles() {
		_, err := credentialStore.CredentialsForRole(arn)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"arn":   arn,
				"error": err.Error(),
			}).Warn("Unable to fetch credential")
		} else {
			logger.WithFields(logrus.Fields{
				"arn": arn,
			}).Info("Successfully fetch credential")
		}
	}
}
