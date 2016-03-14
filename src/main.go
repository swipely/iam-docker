package main

import (
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/swipely/iam-docker/src/app"
	"net/url"
	"os"
	"time"
)

var (
	log = logrus.WithFields(logrus.Fields{"package": "main"})

	listenAddr              = flag.String("listen-addr", ":8080", "Address on which the HTTP server should listen")
	readTimeout             = flag.Duration("read-timeout", time.Minute, "Read timeout of the HTTP server")
	writeTimeout            = flag.Duration("write-timeout", time.Minute, "Write timeout of the HTTP server")
	metadata                = flag.String("meta-data-api", "http://169.254.169.254:80", "Address of the EC2 MetaData API")
	eventHandlers           = flag.Int("event-handlers", 4, "Number of workers listening to the Docker Events channel")
	dockerSyncPeriod        = flag.Duration("docker-sync-period", time.Minute, "Frequency of Docker Container sync")
	credentialRefreshPeriod = flag.Duration("credential-refresh-period", time.Minute, "Frequency of the IAM credential sync")
)

func main() {
	flag.Parse()
	metaDataUpstream, err := url.Parse(*metadata)
	if err != nil {
		log.WithFields(logrus.Fields{
			"url": *metadata,
		}).Error("Invalid EC2 MetaData API URL")
		os.Exit(1)
	}

	config := &app.Config{
		ListenAddr:              *listenAddr,
		MetaDataUpstream:        metaDataUpstream,
		EventHandlers:           *eventHandlers,
		ReadTimeout:             *readTimeout,
		WriteTimeout:            *writeTimeout,
		DockerSyncPeriod:        *dockerSyncPeriod,
		CredentialRefreshPeriod: *credentialRefreshPeriod,
	}
	dockerClient, err := docker.NewClientFromEnv()
	if err != nil {
		log.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Error("Unable to create Docker client from environment, please set DOCKER_HOST")
		os.Exit(1)
	}
	stsClient := sts.New(session.New())
	errChan := make(chan error)

	inst := app.New(config, dockerClient, stsClient, errChan)
	inst.Run()

	err = <-errChan
	log.WithFields(logrus.Fields{
		"error": err.Error(),
	}).Error("Fatal error, exiting")
	os.Exit(1)
}
