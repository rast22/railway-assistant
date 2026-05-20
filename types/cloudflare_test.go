package types

import (
	"encoding/json"
	"testing"
)

func TestCloudflareWorkersBuildToNotificationEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		status    string
		outcome   string
		severity  string
	}{
		{name: "started", eventType: CloudflareWorkersBuildStarted, status: "running", severity: "info"},
		{name: "succeeded", eventType: CloudflareWorkersBuildSucceeded, status: "success", outcome: "success", severity: "info"},
		{name: "failed", eventType: CloudflareWorkersBuildFailed, status: "failed", outcome: "failure", severity: "error"},
		{name: "canceled", eventType: CloudflareWorkersBuildCanceled, status: "canceled", outcome: "canceled", severity: "warning"},
		{name: "cancelled alias", eventType: CloudflareWorkersBuildCancelled, status: "canceled", outcome: "canceled", severity: "warning"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload CloudflareWorkersBuildEvent
			if err := json.Unmarshal([]byte(cloudflareBuildPayload(tt.eventType, tt.status, tt.outcome)), &payload); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}

			event := CloudflareWorkersBuildToNotificationEvent(payload)

			if event.Provider != ProviderCloudflare {
				t.Fatalf("Provider = %q, want %q", event.Provider, ProviderCloudflare)
			}
			if event.SourceID != "my-worker" {
				t.Fatalf("SourceID = %q, want my-worker", event.SourceID)
			}
			if event.Type != tt.eventType {
				t.Fatalf("Type = %q, want %q", event.Type, tt.eventType)
			}
			if event.Status != tt.status {
				t.Fatalf("Status = %q, want %q", event.Status, tt.status)
			}
			if event.Severity != tt.severity {
				t.Fatalf("Severity = %q, want %q", event.Severity, tt.severity)
			}
			if event.Branch != "main" || event.CommitHash != "abc123def456" || event.CommitAuthor != "developer@example.com" {
				t.Fatalf("git metadata not mapped: %#v", event)
			}
			if event.Timestamp.IsZero() {
				t.Fatal("Timestamp is zero")
			}
		})
	}
}

func TestIsCloudflareWorkersBuildEvent(t *testing.T) {
	if !IsCloudflareWorkersBuildEvent(CloudflareWorkersBuildSucceeded) {
		t.Fatal("CloudflareWorkersBuildSucceeded should be supported")
	}
	if !IsCloudflareWorkersBuildEvent(CloudflareWorkersBuildCanceled) {
		t.Fatal("CloudflareWorkersBuildCanceled should be supported")
	}
	if IsCloudflareWorkersBuildEvent("cf.workersBuilds.worker.preview.created") {
		t.Fatal("unexpected event type should not be supported")
	}
}

func cloudflareBuildPayload(eventType, status, outcome string) string {
	if outcome == "" {
		outcome = "null"
	} else {
		outcome = `"` + outcome + `"`
	}
	return `{
		"type": "` + eventType + `",
		"source": {
			"type": "workersBuilds.worker",
			"workerName": "my-worker"
		},
		"payload": {
			"buildUuid": "build-12345678-90ab-cdef-1234-567890abcdef",
			"status": "` + status + `",
			"buildOutcome": ` + outcome + `,
			"createdAt": "2025-05-01T02:48:57.132Z",
			"initializingAt": "2025-05-01T02:48:58.132Z",
			"runningAt": "2025-05-01T02:48:59.132Z",
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
