package utils

import (
	"strings"
	"testing"

	"railway-assistant/types"
)

func TestPrepareTelegramNotificationMessageEscapesMarkdownAndKeepsButtonURL(t *testing.T) {
	event := types.NotificationEvent{
		Provider:        types.ProviderRailway,
		SourceID:        "project-1",
		Type:            "deployment_failed",
		Severity:        "error",
		WorkspaceName:   "Main_Team",
		ProjectName:     "API(Project)",
		EnvironmentName: "Prod-1",
		ServiceName:     "web.api",
		Status:          "failed",
		Branch:          "feature/test",
		CommitAuthor:    "dev_user",
		CommitMessage:   "fix: escape [markdown]!",
		DeploymentURL:   "https://railway.com/deployment",
	}

	message, buttonURL := PrepareTelegramNotificationMessage(event)

	if buttonURL != event.DeploymentURL {
		t.Fatalf("buttonURL = %q, want %q", buttonURL, event.DeploymentURL)
	}
	for _, want := range []string{
		"Main\\_Team",
		"API\\(Project\\)",
		"Prod\\-1",
		"web\\.api",
		"fix: escape \\[markdown\\]\\!",
		"dev\\_user",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("message missing %q:\n%s", want, message)
		}
	}
}
