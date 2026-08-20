package status

import (
	adminservice "github.com/xsyetopz/go-mamacord/internal/adminapi/service"
	"log/slog"
	"net/http"
)

type Responder interface {
	Decode(*http.Request, any) error
	JSON(http.ResponseWriter, int, any)
	Error(http.ResponseWriter, int, string)
	ServiceError(http.ResponseWriter, int, error)
}
type Handler struct {
	service   *adminservice.Service
	logger    *slog.Logger
	responder Responder
}

func New(service *adminservice.Service, logger *slog.Logger, responder Responder) *Handler {
	return &Handler{service: service, logger: logger, responder: responder}
}
