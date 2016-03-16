package http

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/iam"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	iamMethod      = "GET"
	credentialType = "AWS-HMAC"
	credentialCode = "Success"
)

var (
	iamRegex  = regexp.MustCompile("^/[^/]+/meta-data/iam/security-credentials/[^/]+$")
	listRegex = regexp.MustCompile("^/[^/]+/meta-data/iam/security-credentials/?$")
	log       = logrus.WithField("prefix", "http")
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
		"path":       request.URL.Path,
		"method":     request.Method,
		"remoteAddr": request.RemoteAddr,
	})

	if (request.Method == iamMethod) && iamRegex.MatchString(request.URL.Path) {
		logger.Info("Serving IAM credentials request")
		handler.serveIAMRequest(writer, request, logger)
	} else if (request.Method == iamMethod) && listRegex.MatchString(request.URL.Path) {
		logger.Info("Serving list IAM credentials request")
		handler.serveListCredentialsRequest(writer, request, logger)
	} else {
		logger.Debug("Delegating request upstream")
		handler.upstream.ServeHTTP(writer, request)
	}
}

func (handler *httpHandler) serveIAMRequest(writer http.ResponseWriter, request *http.Request, logger *logrus.Entry) {
	role, creds, err := handler.credentialsForAddress(request.RemoteAddr, logger)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Unable to find credentials")
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	splitPath := strings.Split(request.URL.Path, "/")
	requestedRole := splitPath[len(splitPath)-1]
	if !strings.HasSuffix(*role, requestedRole) {
		logger.WithFields(logrus.Fields{
			"actual-role":    *role,
			"requested-role": requestedRole,
		}).Warn("Role mismatch")
		writer.WriteHeader(http.StatusUnauthorized)
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
		logger.WithField("error", err.Error()).Warn("Unable to serialize JSON")
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, err = writer.Write(response)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Unable to write response")
		return
	}
	logger.Info("Successfully responded")
}

func (handler *httpHandler) serveListCredentialsRequest(writer http.ResponseWriter, request *http.Request, logger *logrus.Entry) {
	role, _, err := handler.credentialsForAddress(request.RemoteAddr, logger)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Unable to find credentials")
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	split := strings.Split(*role, "/")
	prettyName := split[len(split)-1]
	_, err = writer.Write([]byte(prettyName))
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Unable to write response")
		return
	}
	logger.Info("Successfully responded")
}

func (handler *httpHandler) credentialsForAddress(address string, logger *logrus.Entry) (*string, *sts.Credentials, error) {
	ip := strings.Split(address, ":")[0]
	logger.Debug("Fetching IAM role")
	role, err := handler.containerStore.IAMRoleForIP(ip)
	if err != nil {
		return nil, nil, err
	}
	logger = logger.WithFields(logrus.Fields{"role": role})
	logger.Debug("Fetching credentials")
	creds, err := handler.credentialStore.CredentialsForRole(role)
	if err != nil {
		return nil, nil, err
	}
	return &role, creds, nil
}

type httpHandler struct {
	upstream        http.Handler
	containerStore  docker.ContainerStore
	credentialStore iam.CredentialStore
}
