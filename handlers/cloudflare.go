package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"railway-assistant/config"
	"railway-assistant/env"
	"railway-assistant/notifications"
	"railway-assistant/types"
)

type CloudflareWorkersBuildsHandler struct {
	Store      *config.Store
	Dispatcher *notifications.Dispatcher
}

func NewCloudflareWorkersBuildsHandler(store *config.Store, dispatcher *notifications.Dispatcher) CloudflareWorkersBuildsHandler {
	return CloudflareWorkersBuildsHandler{Store: store, Dispatcher: dispatcher}
}

func (h CloudflareWorkersBuildsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !validCloudflareToken(r) {
		log.Printf("Rejected cloudflare workers build webhook: invalid token from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Rejected cloudflare workers build webhook: failed to read body from %s: %v", r.RemoteAddr, err)
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	var payload types.CloudflareWorkersBuildEvent
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Rejected cloudflare workers build webhook: invalid JSON from %s: %v", r.RemoteAddr, err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Type == "" {
		log.Printf("Rejected cloudflare workers build webhook: missing type from %s", r.RemoteAddr)
		http.Error(w, "Invalid Payload: missing type", http.StatusBadRequest)
		return
	}
	if !types.IsCloudflareWorkersBuildEvent(payload.Type) {
		log.Printf("Rejected cloudflare workers build webhook: unsupported type=%s from %s", payload.Type, r.RemoteAddr)
		http.Error(w, "Unsupported Cloudflare Workers Builds event", http.StatusBadRequest)
		return
	}
	if payload.Source.WorkerName == "" {
		log.Printf("Rejected cloudflare workers build webhook: missing workerName from %s", r.RemoteAddr)
		http.Error(w, "Invalid Payload: missing source.workerName", http.StatusBadRequest)
		return
	}

	event := types.CloudflareWorkersBuildToNotificationEvent(payload)
	log.Printf(
		"Received cloudflare workers build webhook: type=%s worker=%s status=%q branch=%q",
		event.Type,
		event.SourceID,
		event.Status,
		event.Branch,
	)

	if h.Store != nil {
		if err := h.Store.UpsertKnownProject(config.KnownProject{
			ID:       event.SourceID,
			Provider: config.ProviderCloudflare,
			Name:     event.SourceName,
		}); err != nil {
			log.Printf("Failed to record cloudflare worker: %v\n", err)
		}
	}

	w.WriteHeader(http.StatusOK)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in cloudflare build processing: %v\n", r)
			}
		}()

		if h.Dispatcher == nil {
			return
		}

		result := h.Dispatcher.DispatchTelegram(event)
		log.Printf(
			"Dispatched cloudflare workers build webhook: worker=%s matched_destinations=%d sent=%d legacy_fallback=%t errors=%d",
			event.SourceID,
			result.MatchedDestinations,
			result.Sent,
			result.UsedLegacyFallback,
			len(result.Errors),
		)
		for _, err := range result.Errors {
			log.Printf("Failed to send cloudflare telegram message: %v\n", err)
		}
	}()
}

func validCloudflareToken(r *http.Request) bool {
	expected := env.GetString("CLOUDFLARE_WEBHOOK_TOKEN", "")
	if expected == "" {
		return true
	}

	actual := r.Header.Get("X-Cloudflare-Webhook-Token")
	if actual == "" {
		actual = r.URL.Query().Get("token")
	}

	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
