package status

import (
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	"log/slog"
	"net/http"
)

func (handler *Handler) HandleModulesConfig(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	switch r.Method {
	case http.MethodGet:
		file, err := handler.service.LoadModulesConfig()
		if err != nil {
			handler.responder.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		handler.responder.JSON(w, http.StatusOK, file)
	case http.MethodPut:
		var file config.ModulesFile
		if err := handler.responder.Decode(r, &file); err != nil {
			handler.responder.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := handler.service.SaveModulesConfig(file); err != nil {
			handler.responder.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		handler.logger.Info("admin modules config updated", slog.Uint64("actor_id", sess.UserID))
		handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler *Handler) HandlePermissionsConfig(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	switch r.Method {
	case http.MethodGet:
		file, err := handler.service.LoadPermissionsConfig()
		if err != nil {
			handler.responder.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		handler.responder.JSON(w, http.StatusOK, file)
	case http.MethodPut:
		var file permissions.Policy
		if err := handler.responder.Decode(r, &file); err != nil {
			handler.responder.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := handler.service.SavePermissionsConfig(file); err != nil {
			handler.responder.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		handler.logger.Info("admin permissions config updated", slog.Uint64("actor_id", sess.UserID))
		handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler *Handler) HandleTrustedKeys(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	resp, err := handler.service.TrustedKeys(r.Context())
	if err != nil {
		handler.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	handler.responder.JSON(w, http.StatusOK, resp)
}
