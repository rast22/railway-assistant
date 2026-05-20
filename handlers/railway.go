package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"

	"railway-assistant/config"
	"railway-assistant/env"
	"railway-assistant/notifications"
	"railway-assistant/services"
	"railway-assistant/types"
	"railway-assistant/utils"
)

type RailwayHandler struct {
	Store      *config.Store
	Dispatcher *notifications.Dispatcher
}

func NewRailwayHandler(store *config.Store, dispatcher *notifications.Dispatcher) RailwayHandler {
	return RailwayHandler{Store: store, Dispatcher: dispatcher}
}

func RailwayAlertsHandler(w http.ResponseWriter, r *http.Request) {
	NewRailwayHandler(nil, notifications.NewDispatcher(nil, services.NewTelegramAPIFromEnv())).ServeHTTP(w, r)
}

func (h RailwayHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !validRailwayToken(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload types.RailwayAlert
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Type == "" {
		http.Error(w, "Invalid Payload: missing type", http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in alert processing: %v\n", r)
			}
		}()

		event := types.RailwayAlertToNotificationEvent(payload)
		if h.Store != nil {
			if err := h.Store.UpsertKnownProject(config.KnownProject{
				ID:       event.SourceID,
				Provider: event.Provider,
				Name:     event.SourceName,
			}); err != nil {
				log.Printf("Failed to record railway project: %v\n", err)
			}
		}

		if h.Dispatcher != nil {
			result := h.Dispatcher.DispatchTelegram(event)
			for _, err := range result.Errors {
				log.Printf("Failed to send telegram message: %v\n", err)
			}
		}

		if utils.IsSlackEnabled() {
			slackBlocks := utils.PrepareSlackMessage(payload)
			if err := services.SendSlackMessage(slackBlocks); err != nil {
				log.Printf("Failed to send slack message: %v\n", err)
			}
		}
	}()
}

func validRailwayToken(r *http.Request) bool {
	expected := env.GetString("RAILWAY_WEBHOOK_TOKEN", "")
	if expected == "" {
		return true
	}

	actual := r.Header.Get("X-Railway-Webhook-Token")
	if actual == "" {
		actual = r.URL.Query().Get("token")
	}

	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
