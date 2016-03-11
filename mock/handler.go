package mock

import (
	"net/http"
)

// Handler is a mock http.Handler.
type Handler struct {
	serveHTTP func(writer http.ResponseWriter, request *http.Request)
}

// NewHandler creates a new mock handler.
func NewHandler(serveHTTP func(writer http.ResponseWriter, request *http.Request)) *Handler {
	return &Handler{
		serveHTTP: serveHTTP,
	}
}

func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	handler.serveHTTP(writer, request)
}
