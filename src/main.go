package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	dockerLib "github.com/fsouza/go-dockerclient"
	"github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/http"
	"github.com/swipely/iam-docker/src/iam"
	netHTTP "net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"time"
)

var (
	log = logrus.WithFields(logrus.Fields{"package": "main"})
)

func main() {
	var wg sync.WaitGroup

	stsClient := sts.New(session.New())
	dockerClient, err := dockerLib.NewClientFromEnv()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Error("Unable create Docker client from ENV")
		os.Exit(1)
	}

	url, err := url.Parse("http://169.254.169.254")
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Error("Unable to parse upstream URL")
		os.Exit(1)
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(url)

	containerStore := docker.NewContainerStore(dockerClient)
	credentialStore := iam.NewCredentialStore(stsClient)
	iamHandler := http.NewIAMHandler(reverseProxy, containerStore, credentialStore)

	server := netHTTP.Server{
		Addr:           ":8080",
		Handler:        iamHandler,
		ReadTimeout:    time.Minute,
		WriteTimeout:   time.Minute,
		MaxHeaderBytes: 1 << 20,
	}
	eventHandler := docker.NewEventHandler(4, containerStore, credentialStore)

	eventChannel := make(chan *dockerLib.APIEvents)
	err = dockerClient.AddEventListener(eventChannel)
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err.Error(),
		}).Error("Unable to add docker event listener")
		os.Exit(1)
	}

	wg.Add(1)

	go func() {
		local := log.WithFields(logrus.Fields{"routine": "event-handler"})
		for {
			local.Info("Starting")
			e := eventHandler.Listen(eventChannel)
			local.WithFields(logrus.Fields{
				"error": e.Error(),
			}).Error("Stopped")
			time.Sleep(time.Second)
		}
	}()

	go func() {
		local := log.WithFields(logrus.Fields{"routine": "container-sync"})
		for range time.Tick(time.Minute) {
			local.Info("Starting")
			e := containerStore.SyncRunningContainers()
			if e == nil {
				local.Info("Successfully synced containers")
			} else {
				local.WithFields(logrus.Fields{
					"error": e.Error(),
				}).Error("Failed to sync containers")
			}
		}
	}()

	go func() {
		local := log.WithFields(logrus.Fields{"routine": "http-server"})
		for {
			local.Info("Starting")
			e := server.ListenAndServe()
			local.WithFields(logrus.Fields{
				"error": e.Error(),
			}).Error("Failed to serve HTTP")
			time.Sleep(time.Second)
		}
	}()

	go func() {
		local := log.WithFields(logrus.Fields{"routine": "credential-refresher"})
		time.Sleep(30 * time.Second)
		for range time.Tick(time.Minute) {
			local.Info("Starting")
			credentialStore.RefreshCredentials()
			local.Info("Completed")
		}
	}()

	wg.Wait()
}
