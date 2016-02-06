package http

import (
	"encoding/json"
	"github.com/swipely/iam-docker/docker"
	"github.com/swipely/iam-docker/iam"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"time"
)

const (
	iamMethod      = "GET"
	credentialType = "AWS-HMAC"
	credentialCode = "Success"
)

var (
	iamRegex = regexp.MustCompile("^/[^/]+/meta-data/iam/security-credentials/")
)

// NewIAMHandler creates a http.Handler which responds to metadata API requests.
// When the request is for the IAM path, it looks up the IAM role in the
// container store and fetches those credentials. Otherwise, it acts as a
// reverse proxy for the real API.
func NewIAMHandler(realIAMServer *url.URL, containerStore docker.ContainerStore, credentialStore iam.CredentialStore) http.Handler {
	return &httpHandler{
		containerStore:  containerStore,
		credentialStore: credentialStore,
		reverseProxy:    httputil.NewSingleHostReverseProxy(realIAMServer),
	}
}

func (handler *httpHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if (request.Method == iamMethod) && iamRegex.MatchString(request.URL.Path) {
		handler.serveIAMRequest(writer, request)
	} else {
		handler.reverseProxy.ServeHTTP(writer, request)
	}
}

func (handler *httpHandler) serveIAMRequest(writer http.ResponseWriter, request *http.Request) {
	role, err := handler.containerStore.IAMRoleForIP(request.RemoteAddr)
	if err != nil {
		writer.WriteHeader(http.StatusNotFound)
		return
	}
	creds, err := handler.credentialStore.CredentialsForRole(role)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	response, err := json.Marshal(&credentialResponse{
		AccessKeyID:     *creds.AccessKeyId,
		Code:            credentialCode,
		Expiration:      *creds.Expiration,
		LastUpdated:     creds.Expiration.Add(-1 * time.Hour),
		SecretAccessKey: *creds.SecretAccessKey,
		Type:            credentialType,
		Token:           *creds.SessionToken,
	})
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = writer.Write(response)
}

type credentialResponse struct {
	AccessKeyID     string `json:"AccessKeyId"`
	Code            string
	Expiration      time.Time
	LastUpdated     time.Time
	SecretAccessKey string
	Token           string
	Type            string
}

type httpHandler struct {
	containerStore  docker.ContainerStore
	credentialStore iam.CredentialStore
	reverseProxy    http.Handler
}
