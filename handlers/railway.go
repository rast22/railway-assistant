package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

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
	setRailwayCORSHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !validRailwayToken(r) {
		log.Printf("Rejected railway webhook: invalid token from %s", r.RemoteAddr)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Rejected railway webhook: failed to read body from %s: %v", r.RemoteAddr, err)
		http.Error(w, "Invalid body", http.StatusBadRequest)
		return
	}

	var payload types.RailwayAlert
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Rejected railway webhook: invalid JSON from %s: %v", r.RemoteAddr, err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if payload.Type == "" {
		log.Printf("Rejected railway webhook: missing type from %s", r.RemoteAddr)
		http.Error(w, "Invalid Payload: missing type", http.StatusBadRequest)
		return
	}

	event := types.RailwayAlertToNotificationEvent(payload)
	applyRailwayProjectFallback(r, body, &event)
	log.Printf(
		"Received railway webhook: type=%s project_id=%s project_name=%q service=%q status=%q",
		event.Type,
		event.SourceID,
		event.SourceName,
		event.ServiceName,
		event.Status,
	)

	if h.Store != nil {
		if event.SourceID == "" {
			log.Printf("Railway webhook did not include a project id; add ?project_id=<id> to this project's webhook URL if Railway test payloads omit it")
		} else if err := h.Store.UpsertKnownProject(config.KnownProject{
			ID:       event.SourceID,
			Provider: event.Provider,
			Name:     event.SourceName,
		}); err != nil {
			log.Printf("Failed to record railway project: %v\n", err)
		}
	}

	w.WriteHeader(http.StatusOK)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in alert processing: %v\n", r)
			}
		}()

		if h.Dispatcher != nil {
			result := h.Dispatcher.DispatchTelegram(event)
			log.Printf(
				"Dispatched railway webhook: project_id=%s matched_destinations=%d sent=%d legacy_fallback=%t errors=%d",
				event.SourceID,
				result.MatchedDestinations,
				result.Sent,
				result.UsedLegacyFallback,
				len(result.Errors),
			)
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

func applyRailwayProjectFallback(r *http.Request, body []byte, event *types.NotificationEvent) {
	if event.SourceID == "" {
		event.SourceID = strings.TrimSpace(firstQueryValue(r, "project_id", "projectId", "project"))
	}
	if event.SourceName == "" {
		event.SourceName = strings.TrimSpace(firstQueryValue(r, "project_name", "projectName"))
	}
	if event.ProjectName == "" {
		event.ProjectName = event.SourceName
	}
	if event.SourceID != "" && event.SourceName != "" {
		return
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return
	}

	if event.SourceID == "" {
		event.SourceID = firstString(raw, "projectId", "project_id")
	}
	if event.SourceName == "" {
		event.SourceName = firstString(raw, "projectName", "project_name")
	}

	if project, ok := raw["project"].(map[string]interface{}); ok {
		if event.SourceID == "" {
			event.SourceID = firstString(project, "id", "projectId")
		}
		if event.SourceName == "" {
			event.SourceName = firstString(project, "name", "projectName")
		}
	}

	if event.ProjectName == "" {
		event.ProjectName = event.SourceName
	}
}

func firstQueryValue(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := r.URL.Query().Get(key); value != "" {
			return value
		}
	}
	return ""
}

func firstString(values map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func setRailwayCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Railway-Webhook-Token")
	w.Header().Set("Access-Control-Max-Age", "3600")
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
