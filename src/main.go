package main

import (
	"flag"
	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/swipely/iam-docker/src/app"
	iamLog "github.com/swipely/iam-docker/src/log"
	"net/url"
	"os"
	"time"
)

var (
	listenAddr              = flag.String("listen-addr", ":8080", "Address on which the HTTP server should listen")
	readTimeout             = flag.Duration("read-timeout", time.Minute, "Read timeout of the HTTP server")
	writeTimeout            = flag.Duration("write-timeout", time.Minute, "Write timeout of the HTTP server")
	metadata                = flag.String("meta-data-api", "http://169.254.169.254:80", "Address of the EC2 MetaData API")
	eventHandlers           = flag.Int("event-handlers", 4, "Number of workers listening to the Docker Events channel")
	dockerSyncPeriod        = flag.Duration("docker-sync-period", 0*time.Second, "Frequency of Docker Container sync; default is never")
	credentialRefreshPeriod = flag.Duration("credential-refresh-period", time.Minute, "Frequency of the IAM credential sync")
	verbose                 = flag.Bool("verbose", false, "Enable verbose logging")
)

func main() {
	flag.Parse()

	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	logrus.SetOutput(os.Stdout)
	logrus.SetFormatter(&iamLog.Formatter{})

	log := logrus.WithFields(logrus.Fields{"prefix": "main"})

	metaDataUpstream, err := url.Parse(*metadata)
	if err != nil {
		log.WithField("url", *metadata).Error("Invalid EC2 MetaData API URL")
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
		log.WithField("error", err.Error()).Error("Unable to create Docker client from environment, please set DOCKER_HOST")
		os.Exit(1)
	}
	stsClient := sts.New(session.New())

	inst := app.New(config, dockerClient, stsClient)
	err = inst.Run()
	log.WithField("error", err.Error()).Error("Fatal error, exiting")

	os.Exit(1)
}
