package plugins

import (
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	"log/slog"
	"net/http"
)

func (handler *Handler) HandleModules(w http.ResponseWriter, _ *http.Request, _ adminauth.Session) {
	handler.responder.JSON(w, http.StatusOK, map[string]any{"modules": handler.service.Modules()})
}

func (handler *Handler) HandleSetModule(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		ModuleID string `json:"module_id"`
		Enabled  bool   `json:"enabled"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := handler.service.SetModuleEnabled(r.Context(), req.ModuleID, req.Enabled, sess.UserID); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	handler.logger.Info("admin module state updated", slog.Uint64("actor_id", sess.UserID), slog.String("module_id", req.ModuleID), slog.Bool("enabled", req.Enabled))
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleResetModule(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		ModuleID string `json:"module_id"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := handler.service.ResetModule(r.Context(), req.ModuleID); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	handler.logger.Info("admin module reset", slog.Uint64("actor_id", sess.UserID), slog.String("module_id", req.ModuleID))
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleReloadModules(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	if err := handler.service.ReloadModules(r.Context()); err != nil {
		handler.responder.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	handler.logger.Info("admin modules reloaded", slog.Uint64("actor_id", sess.UserID))
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}
