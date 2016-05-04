package app

import (
	"hash/fnv"
	"net/http/httputil"
	"os"
	"time"

	"github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/http"
	"github.com/swipely/iam-docker/src/iam"
	"github.com/swipely/iam-docker/src/queue"

	"github.com/Sirupsen/logrus"
	dockerLib "github.com/fsouza/go-dockerclient"
	"github.com/valyala/fasthttp"
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
	jobQueue := queue.NewPooledJobQueue(app.Config.QueueSize, app.Config.EventHandlers)
	containerStore := docker.NewContainerStore(app.DockerClient)
	credentialStore := iam.NewCredentialStore(app.STSClient, app.randomSeed())
	eventHandler := docker.NewEventHandler(app.Config.EventHandlers, containerStore, credentialStore)
	proxy := httputil.NewSingleHostReverseProxy(app.Config.MetaDataUpstream)
	handler := http.NewIAMHandler(proxy, containerStore, credentialStore)

	go app.runJobQueue(jobQueue, errorChan)
	go app.scheduleWorker(containerStore, credentialStore, jobQueue)
	go app.httpWorker(handler, errorChan)
	go app.eventWorker(eventHandler, errorChan)

	return <-errorChan
}

func (app *App) runJobQueue(jobQueue queue.JobQueue, errorChan chan error) {
	for {
		err := jobQueue.Run()
		if err != nil {
			log.WithFields(logrus.Fields{
				"worker": "job-queue",
				"err":    err.Error(),
			}).Error("Received error from job queue")
			errorChan <- err
			break
		}
	}
}

func (app *App) scheduleWorker(containerStore docker.ContainerStore, credentialStore iam.CredentialStore, jobQueue queue.JobQueue) {
	dockerTimer := timer(app.Config.DockerSyncPeriod)
	credentialTimer := timer(app.Config.CredentialRefreshPeriod)

	dockerJob := docker.NewSyncContainersJob(app.DockerClient, containerStore, credentialStore, jobQueue)
	credentialJob := iam.NewRefreshCredentialsJob(credentialStore, jobQueue)

	for {
		select {
		case <-dockerTimer:
			jobQueue.Enqueue(dockerJob)
		case <-credentialTimer:
			jobQueue.Enqueue(credentialJob)
		}
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
	_, err = hash.Write([]byte(hostname))
	if err != nil {
		return nano
	}
	return nano ^ int64(hash.Sum64())
}

func timer(duration time.Duration) <-chan time.Time {
	if duration == 0 {
		channel := make(chan time.Time, 1)
		channel <- time.Now()
		return channel
	}

	return time.Tick(duration)
}
