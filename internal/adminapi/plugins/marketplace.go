package plugins

import (
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	adminservice "github.com/xsyetopz/go-mamacord/internal/adminapi/service"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	"net/http"
	"strings"
)

func (handler *Handler) HandleMarketplaceSources(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	switch r.Method {
	case http.MethodGet:
		items, err := handler.service.MarketplaceSources(r.Context())
		if err != nil {
			handler.responder.ServiceError(w, http.StatusInternalServerError, err)
			return
		}
		handler.responder.JSON(w, http.StatusOK, adminservice.MarketplaceSourcesResponse{Sources: items})
	case http.MethodPost:
		var req marketplace.SourceUpsert
		if err := handler.responder.Decode(r, &req); err != nil {
			handler.responder.Error(w, http.StatusBadRequest, err.Error())
			return
		}
		item, err := handler.service.UpsertMarketplaceSource(r.Context(), req)
		if err != nil {
			handler.responder.ServiceError(w, http.StatusBadRequest, err)
			return
		}
		handler.responder.JSON(w, http.StatusOK, item)
	case http.MethodDelete:
		sourceID := strings.TrimSpace(r.URL.Query().Get("source_id"))
		if sourceID == "" {
			handler.responder.Error(w, http.StatusBadRequest, "source_id is required")
			return
		}
		if err := handler.service.DeleteMarketplaceSource(r.Context(), sourceID); err != nil {
			handler.responder.ServiceError(w, http.StatusBadRequest, err)
			return
		}
		handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (handler *Handler) HandleMarketplaceSourceSync(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	var req struct {
		SourceID string `json:"source_id"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := handler.service.SyncMarketplaceSource(r.Context(), req.SourceID)
	if err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, resp)
}

func (handler *Handler) HandleMarketplaceSearch(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	query := marketplace.SearchQuery{
		SourceID: strings.TrimSpace(r.URL.Query().Get("source_id")),
		Term:     strings.TrimSpace(r.URL.Query().Get("term")),
		Refresh:  strings.TrimSpace(r.URL.Query().Get("refresh")) == "1",
	}
	results, err := handler.service.SearchMarketplace(r.Context(), query)
	if err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"results": results})
}

func (handler *Handler) HandleMarketplaceInstall(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req adminservice.MarketplaceInstallRequest
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := handler.service.InstallMarketplacePlugin(r.Context(), sess.UserID, req)
	if err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, resp)
}

func (handler *Handler) HandleMarketplaceUpdate(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req adminservice.MarketplaceUpdateRequest
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := handler.service.UpdateMarketplacePlugin(r.Context(), sess.UserID, req)
	if err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, resp)
}

func (handler *Handler) HandleMarketplaceUninstall(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	var req adminservice.MarketplaceUninstallRequest
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := handler.service.UninstallMarketplacePlugin(r.Context(), req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleMarketplaceTrustSigner(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	var req adminservice.MarketplaceTrustSignerRequest
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := handler.service.TrustMarketplaceSigner(r.Context(), req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleMarketplaceTrustVendor(w http.ResponseWriter, r *http.Request, _ adminauth.Session) {
	var req adminservice.MarketplaceTrustVendorRequest
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := handler.service.TrustMarketplaceVendor(r.Context(), req)
	if err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, resp)
}
