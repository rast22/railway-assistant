package handlers

import (
	"net/http"

	"railway-assistant/config"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func NewHealthHandler(store *config.Store, configRequired bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if configRequired && (store == nil || !store.Ready()) {
			http.Error(w, "Config store is not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}
