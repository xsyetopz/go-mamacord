package plugins

import (
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	adminservice "github.com/xsyetopz/go-mamacord/internal/adminapi/service"
	"log/slog"
	"net/http"
)

func (handler *Handler) HandlePlugins(w http.ResponseWriter, _ *http.Request, _ adminauth.Session) {
	plugins, err := handler.service.Plugins()
	if err != nil {
		handler.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"plugins": plugins})
}

func (handler *Handler) HandleReloadPlugins(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	if err := handler.service.ReloadPlugins(r.Context()); err != nil {
		handler.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	handler.logger.Info("admin plugins reloaded", slog.Uint64("actor_id", sess.UserID))
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleScaffoldPlugin(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req adminservice.PluginScaffoldRequest
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := handler.service.ScaffoldPlugin(req)
	if err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	handler.logger.Info("admin plugin scaffolded", slog.Uint64("actor_id", sess.UserID), slog.String("plugin_id", resp.ID))
	handler.responder.JSON(w, http.StatusOK, resp)
}

func (handler *Handler) HandleSignPlugin(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		PluginID string `json:"plugin_id"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	path, err := handler.service.SignPlugin(req.PluginID)
	if err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	handler.logger.Info("admin plugin signed", slog.Uint64("actor_id", sess.UserID), slog.String("plugin_id", req.PluginID))
	handler.responder.JSON(w, http.StatusOK, map[string]any{"signature": path})
}
