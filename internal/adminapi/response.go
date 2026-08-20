package adminapi

import (
	"encoding/json"
	"errors"
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
	var pe interface {
		error
		StatusCode() int
		PublicMessage() string
		RetryDelay() time.Duration
	}
	if errors.As(err, &pe) && pe != nil {
		status := pe.StatusCode()
		retryAfter := pe.RetryDelay()
		payload := map[string]any{"error": strings.TrimSpace(pe.PublicMessage())}
		if retryAfter > 0 {
			retrySeconds := int64(retryAfter.Round(time.Second).Seconds())
			if retrySeconds < 1 {
				retrySeconds = 1
			}
			w.Header().Set("Retry-After", strconv.FormatInt(retrySeconds, 10))
			payload["retry_after_ms"] = int64(retryAfter.Round(time.Millisecond) / time.Millisecond)
		}
		writeJSON(w, status, payload)
		return
	}
	writeError(w, fallbackStatus, err.Error())
}

type httpResponder struct{}

func (httpResponder) Decode(r *http.Request, dst any) error { return decodeJSON(r, dst) }
func (httpResponder) JSON(w http.ResponseWriter, status int, payload any) {
	writeJSON(w, status, payload)
}
func (httpResponder) Error(w http.ResponseWriter, status int, message string) {
	writeError(w, status, message)
}
func (httpResponder) ServiceError(w http.ResponseWriter, status int, err error) {
	writeServiceError(w, status, err)
}
