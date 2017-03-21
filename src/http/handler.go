package http

import (
	"encoding/json"
	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/swipely/iam-docker/src/docker"
	"github.com/swipely/iam-docker/src/iam"
	"github.com/valyala/fasthttp"
	adaptor "github.com/valyala/fasthttp/fasthttpadaptor"
	"net/http"
	"strings"
	"time"
)

const (
	iamMethod      = "GET"
	credentialType = "AWS-HMAC"
	credentialCode = "Success"
	iamPath        = "/meta-data/iam/security-credentials/"
	healthMethod   = "GET"
	healthPath     = "/healthcheck"
)

var (
	log = logrus.WithField("prefix", "http")
)

// NewIAMHandler creates a http.Handler which responds to metadata API requests.
// When the request is for the IAM path, it looks up the IAM role in the
// container store and fetches those credentials. Otherwise, it acts as a
// reverse proxy for the real API.
func NewIAMHandler(upstream http.Handler, containerStore docker.ContainerStore, credentialStore iam.CredentialStore, disableUpstream bool) fasthttp.RequestHandler {
	handler := &httpHandler{
		upstreamHandler: adaptor.NewFastHTTPHandler(upstream),
		containerStore:  containerStore,
		credentialStore: credentialStore,
		disableUpstream: disableUpstream,
	}

	return handler.serveFastHTTP
}

func (handler *httpHandler) serveFastHTTP(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	method := string(ctx.Method())
	addr := ctx.RemoteAddr().String()

	logger := log.WithFields(logrus.Fields{
		"path":       path,
		"method":     method,
		"remoteAddr": addr,
	})

	if method == healthMethod && path == healthPath {
		logger.Debug("Serving health check request")
		handler.serveHealthRequest(ctx, logger)
		return
	} else if method == iamMethod {
		idx := strings.LastIndex(path, iamPath)
		if idx == (len(path) - len(iamPath)) {
			logger.Debug("Serving list IAM credentials request")
			handler.serveListCredentialsRequest(ctx, addr, logger)
			return
		} else if idx >= 0 {
			logger.Info("Serving IAM credentials request")
			handler.serveIAMRequest(ctx, addr, path, logger)
			return
		} else if handler.disableUpstream {
			logger.Info("Denying non-IAM endpoint request")
			handler.serveDeniedRequest(ctx, addr, path, logger)
			return
		}
	} else if handler.disableUpstream {
		logger.Info("Denying non-IAM endpoint request")
		handler.serveDeniedRequest(ctx, addr, path, logger)
		return
	}

	logger.Debug("Delegating request upstream")
	handler.upstreamHandler(ctx)
}

func (handler *httpHandler) serveIAMRequest(ctx *fasthttp.RequestCtx, addr string, path string, logger *logrus.Entry) {
	role, creds, err := handler.credentialsForAddress(addr)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Unable to find credentials")
		ctx.SetStatusCode(http.StatusNotFound)
		return
	}
	idx := strings.LastIndex(path, "/")
	requestedRole := path[idx+1:]
	if !strings.HasSuffix(*role, requestedRole) {
		logger.WithFields(logrus.Fields{
			"actual-role":    *role,
			"requested-role": requestedRole,
		}).Warn("Role mismatch")
		ctx.SetStatusCode(http.StatusUnauthorized)
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
		ctx.SetStatusCode(http.StatusInternalServerError)
		return
	}
	ctx.SetBody(response)
	logger.Debug("Successfully responded")
}

func (handler *httpHandler) serveListCredentialsRequest(ctx *fasthttp.RequestCtx, addr string, logger *logrus.Entry) {
	role, _, err := handler.credentialsForAddress(addr)
	if err != nil {
		logger.WithField("error", err.Error()).Warn("Unable to find credentials")
		ctx.SetStatusCode(http.StatusNotFound)
		return
	}
	idx := strings.LastIndex(*role, "/")
	ctx.SetBodyString((*role)[idx+1:])
	logger.Debug("Successfully responded")
}

func (handler *httpHandler) serveDeniedRequest(ctx *fasthttp.RequestCtx, addr string, path string, logger *logrus.Entry) {
	ctx.SetStatusCode(403)
	logger.Debug("Successfully responded")
}

func (handler *httpHandler) serveHealthRequest(ctx *fasthttp.RequestCtx, logger *logrus.Entry) {
	ctx.SetStatusCode(200)
	logger.Debug("Successfully responded")
}

func (handler *httpHandler) credentialsForAddress(address string) (*string, *sts.Credentials, error) {
	ip := strings.Split(address, ":")[0]
	role, err := handler.containerStore.IAMRoleForIP(ip)
	if err != nil {
		return nil, nil, err
	}
	creds, err := handler.credentialStore.CredentialsForRole(role.Arn, role.ExternalId)
	if err != nil {
		return nil, nil, err
	}
	return &role.Arn, creds, nil
}

type httpHandler struct {
	upstreamHandler fasthttp.RequestHandler
	containerStore  docker.ContainerStore
	credentialStore iam.CredentialStore
	disableUpstream bool
}
