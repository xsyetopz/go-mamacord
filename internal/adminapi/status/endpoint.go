package status

import (
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	"net/http"
)

func (handler *Handler) HandleStatus(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	resp, err := handler.service.Status(r.Context())
	if err != nil {
		handler.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	handler.responder.JSON(w, http.StatusOK, resp)
}
