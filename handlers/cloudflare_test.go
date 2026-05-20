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
	"railway-assistant/types"
)

func TestCloudflareWorkersBuildsHandlerRecordsKnownWorkerAndDispatches(t *testing.T) {
	t.Setenv("CLOUDFLARE_WEBHOOK_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")

	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if _, err := store.ConnectTelegramDestination(config.ProviderCloudflare, "my-worker", config.TelegramDestination{ChatID: "-1001"}, 42); err != nil {
		t.Fatalf("ConnectTelegramDestination() error = %v", err)
	}

	sender := &handlerTelegramSender{messages: make(chan handlerTelegramMessage, 1)}
	handler := NewCloudflareWorkersBuildsHandler(store, notifications.NewDispatcher(store, sender))
	req := httptest.NewRequest(http.MethodPost, "/cloudflare/workers/builds", strings.NewReader(cloudflareHandlerPayload(types.CloudflareWorkersBuildSucceeded)))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	project, ok := store.KnownProject(config.ProviderCloudflare, "my-worker")
	if !ok {
		t.Fatal("cloudflare worker was not recorded")
	}
	if project.Name != "my-worker" {
		t.Fatalf("worker name = %q, want my-worker", project.Name)
	}

	select {
	case message := <-sender.messages:
		if message.chatID != "-1001" {
			t.Fatalf("sent chat = %q, want -1001", message.chatID)
		}
		if !strings.Contains(message.message.Text, "Cloudflare Workers Build") {
			t.Fatalf("message text missing Cloudflare title:\n%s", message.message.Text)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for telegram send")
	}
}

func TestCloudflareWorkersBuildsHandlerAcceptsValidPayloadWithoutRoute(t *testing.T) {
	t.Setenv("CLOUDFLARE_WEBHOOK_TOKEN", "")
	t.Setenv("TELEGRAM_CHAT_ID", "")

	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	sender := &handlerTelegramSender{messages: make(chan handlerTelegramMessage, 1)}
	handler := NewCloudflareWorkersBuildsHandler(store, notifications.NewDispatcher(store, sender))
	req := httptest.NewRequest(http.MethodPost, "/cloudflare/workers/builds", strings.NewReader(cloudflareHandlerPayload(types.CloudflareWorkersBuildStarted)))
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

func TestCloudflareWorkersBuildsHandlerRejectsInvalidRequests(t *testing.T) {
	t.Setenv("CLOUDFLARE_WEBHOOK_TOKEN", "secret")
	t.Setenv("TELEGRAM_CHAT_ID", "")

	handler := NewCloudflareWorkersBuildsHandler(nil, nil)

	tests := []struct {
		name   string
		method string
		url    string
		body   string
		status int
		header string
	}{
		{name: "method", method: http.MethodGet, url: "/cloudflare/workers/builds", status: http.StatusMethodNotAllowed},
		{name: "missing token", method: http.MethodPost, url: "/cloudflare/workers/builds", body: `{}`, status: http.StatusUnauthorized},
		{name: "invalid json", method: http.MethodPost, url: "/cloudflare/workers/builds", body: `{`, status: http.StatusBadRequest, header: "secret"},
		{name: "missing type", method: http.MethodPost, url: "/cloudflare/workers/builds", body: `{"source":{"workerName":"my-worker"}}`, status: http.StatusBadRequest, header: "secret"},
		{name: "unsupported type", method: http.MethodPost, url: "/cloudflare/workers/builds", body: `{"type":"cf.other","source":{"workerName":"my-worker"}}`, status: http.StatusBadRequest, header: "secret"},
		{name: "missing worker", method: http.MethodPost, url: "/cloudflare/workers/builds", body: `{"type":"cf.workersBuilds.worker.build.succeeded"}`, status: http.StatusBadRequest, header: "secret"},
		{name: "query token", method: http.MethodPost, url: "/cloudflare/workers/builds?token=secret", body: cloudflareHandlerPayload(types.CloudflareWorkersBuildSucceeded), status: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.url, strings.NewReader(tt.body))
			if tt.header != "" {
				req.Header.Set("X-Cloudflare-Webhook-Token", tt.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d", rec.Code, tt.status)
			}
		})
	}
}

func cloudflareHandlerPayload(eventType string) string {
	return `{
		"type": "` + eventType + `",
		"source": {
			"type": "workersBuilds.worker",
			"workerName": "my-worker"
		},
		"payload": {
			"buildUuid": "build-12345678-90ab-cdef-1234-567890abcdef",
			"status": "success",
			"buildOutcome": "success",
			"createdAt": "2025-05-01T02:48:57.132Z",
			"stoppedAt": "2025-05-01T02:50:15.132Z",
			"buildTriggerMetadata": {
				"buildTriggerSource": "push_event",
				"branch": "main",
				"commitHash": "abc123def456",
				"commitMessage": "Fix bug in authentication",
				"author": "developer@example.com",
				"buildCommand": "npm run build",
				"deployCommand": "wrangler deploy",
				"rootDirectory": "/",
				"repoName": "my-worker-repo",
				"providerAccountName": "github-user",
				"providerType": "github"
			}
		},
		"metadata": {
			"accountId": "f9f79265f388666de8122cfb508d7776",
			"eventSubscriptionId": "1830c4bb612e43c3af7f4cada31fbf3f",
			"eventSchemaVersion": 1,
			"eventTimestamp": "2025-05-01T02:48:57.132Z"
		}
	}`
}
