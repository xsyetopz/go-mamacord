package adminapi

import (
	"net/http"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	"log/slog"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, _ session) {
	resp, err := s.svc.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleModules(w http.ResponseWriter, _ *http.Request, _ session) {
	writeJSON(w, http.StatusOK, map[string]any{"modules": s.svc.Modules()})
}

func (s *Server) handleSetModule(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		ModuleID string `json:"module_id"`
		Enabled  bool   `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.SetModuleEnabled(r.Context(), req.ModuleID, req.Enabled, sess.UserID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin module state updated", slog.Uint64("actor_id", sess.UserID), slog.String("module_id", req.ModuleID), slog.Bool("enabled", req.Enabled))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleResetModule(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		ModuleID string `json:"module_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.ResetModule(r.Context(), req.ModuleID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin module reset", slog.Uint64("actor_id", sess.UserID), slog.String("module_id", req.ModuleID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleReloadModules(w http.ResponseWriter, r *http.Request, sess session) {
	if err := s.svc.ReloadModules(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("admin modules reloaded", slog.Uint64("actor_id", sess.UserID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handlePlugins(w http.ResponseWriter, _ *http.Request, _ session) {
	plugins, err := s.svc.Plugins()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"plugins": plugins})
}

func (s *Server) handleReloadPlugins(w http.ResponseWriter, r *http.Request, sess session) {
	if err := s.svc.ReloadPlugins(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("admin plugins reloaded", slog.Uint64("actor_id", sess.UserID))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleScaffoldPlugin(w http.ResponseWriter, r *http.Request, sess session) {
	var req PluginScaffoldRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.ScaffoldPlugin(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin plugin scaffolded", slog.Uint64("actor_id", sess.UserID), slog.String("plugin_id", resp.ID))
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSignPlugin(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		PluginID string `json:"plugin_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	path, err := s.svc.SignPlugin(req.PluginID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Info("admin plugin signed", slog.Uint64("actor_id", sess.UserID), slog.String("plugin_id", req.PluginID))
	writeJSON(w, http.StatusOK, map[string]any{"signature": path})
}

func (s *Server) handleMarketplaceSources(w http.ResponseWriter, r *http.Request, _ session) {
	switch r.Method {
	case http.MethodGet:
		items, err := s.svc.MarketplaceSources(r.Context())
		if err != nil {
			writeServiceError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, MarketplaceSourcesResponse{Sources: items})
	case http.MethodPost:
		var req marketplace.SourceUpsert
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := s.svc.UpsertMarketplaceSource(r.Context(), req)
		if err != nil {
			writeServiceError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		sourceID := strings.TrimSpace(r.URL.Query().Get("source_id"))
		if sourceID == "" {
			writeError(w, http.StatusBadRequest, "source_id is required")
			return
		}
		if err := s.svc.DeleteMarketplaceSource(r.Context(), sourceID); err != nil {
			writeServiceError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleMarketplaceSourceSync(w http.ResponseWriter, r *http.Request, _ session) {
	var req struct {
		SourceID string `json:"source_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.SyncMarketplaceSource(r.Context(), req.SourceID)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMarketplaceSearch(w http.ResponseWriter, r *http.Request, _ session) {
	query := marketplace.SearchQuery{
		SourceID: strings.TrimSpace(r.URL.Query().Get("source_id")),
		Term:     strings.TrimSpace(r.URL.Query().Get("term")),
		Refresh:  strings.TrimSpace(r.URL.Query().Get("refresh")) == "1",
	}
	results, err := s.svc.SearchMarketplace(r.Context(), query)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Server) handleMarketplaceInstall(w http.ResponseWriter, r *http.Request, sess session) {
	var req MarketplaceInstallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.InstallMarketplacePlugin(r.Context(), sess.UserID, req)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMarketplaceUpdate(w http.ResponseWriter, r *http.Request, sess session) {
	var req MarketplaceUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.UpdateMarketplacePlugin(r.Context(), sess.UserID, req)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMarketplaceUninstall(w http.ResponseWriter, r *http.Request, _ session) {
	var req MarketplaceUninstallRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.UninstallMarketplacePlugin(r.Context(), req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMarketplaceTrustSigner(w http.ResponseWriter, r *http.Request, _ session) {
	var req MarketplaceTrustSignerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.svc.TrustMarketplaceSigner(r.Context(), req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleMarketplaceTrustVendor(w http.ResponseWriter, r *http.Request, _ session) {
	var req MarketplaceTrustVendorRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.svc.TrustMarketplaceVendor(r.Context(), req)
	if err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleModulesConfig(w http.ResponseWriter, r *http.Request, sess session) {
	switch r.Method {
	case http.MethodGet:
		file, err := s.svc.LoadModulesConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, file)
	case http.MethodPut:
		var file config.ModulesFile
		if err := decodeJSON(r, &file); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.svc.SaveModulesConfig(file); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.logger.Info("admin modules config updated", slog.Uint64("actor_id", sess.UserID))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handlePermissionsConfig(w http.ResponseWriter, r *http.Request, sess session) {
	switch r.Method {
	case http.MethodGet:
		file, err := s.svc.LoadPermissionsConfig()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, file)
	case http.MethodPut:
		var file permissions.Policy
		if err := decodeJSON(r, &file); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := s.svc.SavePermissionsConfig(file); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.logger.Info("admin permissions config updated", slog.Uint64("actor_id", sess.UserID))
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTrustedKeys(w http.ResponseWriter, r *http.Request, _ session) {
	resp, err := s.svc.TrustedKeys(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleMigrationStatus(w http.ResponseWriter, r *http.Request, _ session) {
	status, err := s.svc.MigrationStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleMigrationUp(w http.ResponseWriter, r *http.Request, sess session) {
	status, err := s.svc.MigrateUp(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Info("admin migrations applied", slog.Uint64("actor_id", sess.UserID), slog.Int("version", status.CurrentVersion))
	writeJSON(w, http.StatusOK, status)
}
