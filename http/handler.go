package http

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/swipely/iam-docker/docker"
	"github.com/swipely/iam-docker/iam"
	"net/http"
	"regexp"
	"time"
)

const (
	iamMethod      = "GET"
	credentialType = "AWS-HMAC"
	credentialCode = "Success"
)

var (
	iamRegex = regexp.MustCompile("^/[^/]+/meta-data/iam/security-credentials")
	log      = logrus.WithFields(logrus.Fields{"package": "http"})
)

// NewIAMHandler creates a http.Handler which responds to metadata API requests.
// When the request is for the IAM path, it looks up the IAM role in the
// container store and fetches those credentials. Otherwise, it acts as a
// reverse proxy for the real API.
func NewIAMHandler(upstream http.Handler, containerStore docker.ContainerStore, credentialStore iam.CredentialStore) http.Handler {
	return &httpHandler{
		upstream:        upstream,
		containerStore:  containerStore,
		credentialStore: credentialStore,
	}
}

func (handler *httpHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	logger := log.WithFields(logrus.Fields{
		"path":   request.URL.Path,
		"method": request.Method,
	})
	if (request.Method == iamMethod) && iamRegex.MatchString(request.URL.Path) {
		logger.Info("Serving IAM credentials request")
		handler.serveIAMRequest(writer, request, logger)
	} else {
		logger.Info("Delegating request upstream")
		handler.upstream.ServeHTTP(writer, request)
	}
}

func (handler *httpHandler) serveIAMRequest(writer http.ResponseWriter, request *http.Request, logger *logrus.Entry) {
	logger = logger.WithFields(logrus.Fields{"remoteAddr": request.RemoteAddr})
	logger.Debug("Fetching IAM role")
	role, err := handler.containerStore.IAMRoleForIP(request.RemoteAddr)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("Unable to find IAM role")
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	logger = logger.WithFields(logrus.Fields{"role": role})
	logger.Debug("Fetching credentials")
	creds, err := handler.credentialStore.CredentialsForRole(role)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("Unable to find credentials")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	response, err := json.Marshal(&CredentialResponse{
		AccessKeyID:     *creds.AccessKeyId,
		Code:            credentialCode,
		Expiration:      *creds.Expiration,
		LastUpdated:     creds.Expiration.Add(-1 * time.Hour),
		SecretAccessKey: *creds.SecretAccessKey,
		Token:           *creds.SessionToken,
		Type:            credentialType,
	})
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("Unable to serialize JSON")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = writer.Write(response)
	if err != nil {
		logger.WithFields(logrus.Fields{
			"error": err.Error(),
		}).Warn("Unable to write response")
		return
	}
	logger.Info("Successfully responded")
}

type httpHandler struct {
	upstream        http.Handler
	containerStore  docker.ContainerStore
	credentialStore iam.CredentialStore
}
