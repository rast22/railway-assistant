package handlers

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"railway-assistant/config"
	"railway-assistant/notifications"
	"railway-assistant/services"
)

type handlerTelegramSender struct {
	messages chan handlerTelegramMessage
}

type handlerTelegramMessage struct {
	chatID  string
	message services.TelegramMessage
}

func (s *handlerTelegramSender) SendMessage(chatID string, message services.TelegramMessage) error {
	s.messages <- handlerTelegramMessage{chatID: chatID, message: message}
	return nil
}

func (s *handlerTelegramSender) AnswerCallbackQuery(callbackID, text string) error {
	return nil
}

func TestRailwayHandlerSendsToConfiguredProjectRoute(t *testing.T) {
	t.Setenv("RAILWAY_WEBHOOK_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("SLACK_ENABLED", "false")

	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if _, err := store.ConnectTelegramDestination(config.ProviderRailway, "project-1", config.TelegramDestination{ChatID: "-1001"}, 42); err != nil {
		t.Fatalf("ConnectTelegramDestination() error = %v", err)
	}

	sender := &handlerTelegramSender{messages: make(chan handlerTelegramMessage, 1)}
	handler := NewRailwayHandler(store, notifications.NewDispatcher(store, sender))
	req := httptest.NewRequest(http.MethodPost, "/railway/alerts", strings.NewReader(railwayPayload("project-1", "API")))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	select {
	case message := <-sender.messages:
		if message.chatID != "-1001" {
			t.Fatalf("sent chat = %q, want -1001", message.chatID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for telegram send")
	}

	project, ok := store.KnownProject(config.ProviderRailway, "project-1")
	if !ok {
		t.Fatal("known project was not recorded")
	}
	if project.Name != "API" {
		t.Fatalf("project name = %q, want API", project.Name)
	}
}

func TestRailwayHandlerAcceptsValidPayloadWithoutRoute(t *testing.T) {
	t.Setenv("RAILWAY_WEBHOOK_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("SLACK_ENABLED", "false")

	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	sender := &handlerTelegramSender{messages: make(chan handlerTelegramMessage, 1)}
	handler := NewRailwayHandler(store, notifications.NewDispatcher(store, sender))
	req := httptest.NewRequest(http.MethodPost, "/railway/alerts", strings.NewReader(railwayPayload("project-2", "Worker")))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	select {
	case message := <-sender.messages:
		t.Fatalf("unexpected telegram send: %#v", message)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRailwayHandlerRecordsProjectFromQueryFallback(t *testing.T) {
	t.Setenv("RAILWAY_WEBHOOK_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("SLACK_ENABLED", "false")

	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	handler := NewRailwayHandler(store, nil)
	req := httptest.NewRequest(http.MethodPost, "/railway/alerts?project_id=query-project&project_name=Query%20Project", strings.NewReader(`{"type":"test"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	project, ok := store.KnownProject(config.ProviderRailway, "query-project")
	if !ok {
		t.Fatal("query fallback project was not recorded")
	}
	if project.Name != "Query Project" {
		t.Fatalf("project name = %q, want Query Project", project.Name)
	}
}

func TestRailwayHandlerRecordsProjectFromTopLevelPayloadFallback(t *testing.T) {
	t.Setenv("RAILWAY_WEBHOOK_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("SLACK_ENABLED", "false")

	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	handler := NewRailwayHandler(store, nil)
	req := httptest.NewRequest(http.MethodPost, "/railway/alerts", strings.NewReader(`{"type":"test","projectId":"top-project","projectName":"Top Project"}`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	project, ok := store.KnownProject(config.ProviderRailway, "top-project")
	if !ok {
		t.Fatal("top-level fallback project was not recorded")
	}
	if project.Name != "Top Project" {
		t.Fatalf("project name = %q, want Top Project", project.Name)
	}
}

func TestRailwayHandlerSupportsRailwayTestWebhookPreflight(t *testing.T) {
	t.Setenv("RAILWAY_WEBHOOK_TOKEN", "secret")

	handler := NewRailwayHandler(nil, nil)
	req := httptest.NewRequest(http.MethodOptions, "/railway/alerts?token=secret", nil)
	req.Header.Set("Origin", "https://railway.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "content-type")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPost) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want POST", got)
	}
}

func TestRailwayHandlerRejectsInvalidRequests(t *testing.T) {
	t.Setenv("RAILWAY_WEBHOOK_TOKEN", "secret")
	t.Setenv("TELEGRAM_CHAT_ID", "")
	t.Setenv("SLACK_ENABLED", "false")

	handler := NewRailwayHandler(nil, nil)

	tests := []struct {
		name   string
		method string
		url    string
		body   string
		status int
		header string
	}{
		{name: "method", method: http.MethodGet, url: "/railway/alerts", status: http.StatusMethodNotAllowed},
		{name: "missing token", method: http.MethodPost, url: "/railway/alerts", body: `{}`, status: http.StatusUnauthorized},
		{name: "invalid json", method: http.MethodPost, url: "/railway/alerts", body: `{`, status: http.StatusBadRequest, header: "secret"},
		{name: "missing type", method: http.MethodPost, url: "/railway/alerts", body: `{"resource":{"project":{"id":"p"}}}`, status: http.StatusBadRequest, header: "secret"},
		{name: "query token", method: http.MethodPost, url: "/railway/alerts?token=secret", body: railwayPayload("p", "P"), status: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, strings.NewReader(tt.body))
			if tt.header != "" {
				req.Header.Set("X-Railway-Webhook-Token", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
		})
	}
}

func railwayPayload(projectID, projectName string) string {
	return `{
		"type": "deployment_success",
		"severity": "info",
		"timestamp": "2026-05-20T12:00:00Z",
		"resource": {
			"workspace": {"id": "workspace-1", "name": "Main"},
			"project": {"id": "` + projectID + `", "name": "` + projectName + `"},
			"environment": {"id": "environment-1", "name": "Production", "isEphemeral": false},
			"service": {"id": "service-1", "name": "API"},
			"deployment": {"id": "deployment-1"}
		},
		"details": {
			"id": "deployment-1",
			"branch": "main",
			"status": "success",
			"commitAuthor": "dev",
			"commitMessage": "deploy"
		}
	}`
}
