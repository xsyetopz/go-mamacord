package adminapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": strings.TrimSpace(message)})
}

func writeServiceError(w http.ResponseWriter, fallbackStatus int, err error) {
	if err == nil {
		return
	}
	if pe, ok := asPublicError(err); ok {
		status := pe.statusCode()
		payload := map[string]any{"error": strings.TrimSpace(pe.Message)}
		if pe.RetryAfter > 0 {
			retrySeconds := int64(pe.RetryAfter.Round(time.Second).Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", strconv.FormatInt(retrySeconds, 10))
			payload["retry_after_ms"] = int64(pe.RetryAfter.Round(time.Millisecond) / time.Millisecond)
		}
		writeJSON(w, status, payload)
		return
	}
	writeError(w, fallbackStatus, err.Error())
}
