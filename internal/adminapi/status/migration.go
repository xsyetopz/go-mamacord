package status

import (
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	"log/slog"
	"net/http"
)

func (handler *Handler) HandleMigrationStatus(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	status, err := handler.service.MigrationStatus(r.Context())
	if err != nil {
		handler.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	handler.responder.JSON(w, http.StatusOK, status)
}

func (handler *Handler) HandleMigrationUp(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	status, err := handler.service.MigrateUp(r.Context())
	if err != nil {
		handler.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	handler.logger.Info("admin migrations applied", slog.Uint64("actor_id", sess.UserID), slog.Int("version", status.CurrentVersion))
	handler.responder.JSON(w, http.StatusOK, status)
}
