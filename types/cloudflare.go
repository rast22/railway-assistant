package types

import (
	"strings"
	"time"
)

const (
	CloudflareWorkersBuildStarted   = "cf.workersBuilds.worker.build.started"
	CloudflareWorkersBuildSucceeded = "cf.workersBuilds.worker.build.succeeded"
	CloudflareWorkersBuildFailed    = "cf.workersBuilds.worker.build.failed"
	CloudflareWorkersBuildCanceled  = "cf.workersBuilds.worker.build.canceled"
	CloudflareWorkersBuildCancelled = "cf.workersBuilds.worker.build.cancelled"
)

type CloudflareWorkersBuildEvent struct {
	Type     string                     `json:"type"`
	Source   CloudflareWorkersSource    `json:"source"`
	Payload  CloudflareWorkersPayload   `json:"payload"`
	Metadata CloudflareWorkersEventMeta `json:"metadata"`
}

type CloudflareWorkersSource struct {
	Type       string `json:"type"`
	WorkerName string `json:"workerName"`
}

type CloudflareWorkersPayload struct {
	BuildUUID            string                         `json:"buildUuid"`
	Status               string                         `json:"status"`
	BuildOutcome         string                         `json:"buildOutcome"`
	CreatedAt            time.Time                      `json:"createdAt"`
	InitializingAt       *time.Time                     `json:"initializingAt"`
	RunningAt            *time.Time                     `json:"runningAt"`
	StoppedAt            *time.Time                     `json:"stoppedAt"`
	BuildTriggerMetadata CloudflareBuildTriggerMetadata `json:"buildTriggerMetadata"`
}

type CloudflareBuildTriggerMetadata struct {
	BuildTriggerSource  string `json:"buildTriggerSource"`
	Branch              string `json:"branch"`
	CommitHash          string `json:"commitHash"`
	CommitMessage       string `json:"commitMessage"`
	Author              string `json:"author"`
	BuildCommand        string `json:"buildCommand"`
	DeployCommand       string `json:"deployCommand"`
	RootDirectory       string `json:"rootDirectory"`
	RepoName            string `json:"repoName"`
	ProviderAccountName string `json:"providerAccountName"`
	ProviderType        string `json:"providerType"`
}

type CloudflareWorkersEventMeta struct {
	AccountID           string    `json:"accountId"`
	EventSubscriptionID string    `json:"eventSubscriptionId"`
	EventSchemaVersion  int       `json:"eventSchemaVersion"`
	EventTimestamp      time.Time `json:"eventTimestamp"`
}

func IsCloudflareWorkersBuildEvent(eventType string) bool {
	switch eventType {
	case CloudflareWorkersBuildStarted,
		CloudflareWorkersBuildSucceeded,
		CloudflareWorkersBuildFailed,
		CloudflareWorkersBuildCanceled,
		CloudflareWorkersBuildCancelled:
		return true
	default:
		return false
	}
}

func CloudflareWorkersBuildToNotificationEvent(payload CloudflareWorkersBuildEvent) NotificationEvent {
	trigger := payload.Payload.BuildTriggerMetadata
	timestamp := payload.Metadata.EventTimestamp
	if timestamp.IsZero() {
		timestamp = payload.Payload.CreatedAt
	}

	status := strings.TrimSpace(payload.Payload.Status)
	if status == "" {
		status = payload.Payload.BuildOutcome
	}

	severity := "info"
	switch payload.Type {
	case CloudflareWorkersBuildFailed:
		severity = "error"
	case CloudflareWorkersBuildCanceled, CloudflareWorkersBuildCancelled:
		severity = "warning"
	}

	return NotificationEvent{
		Provider:      ProviderCloudflare,
		SourceID:      payload.Source.WorkerName,
		SourceName:    payload.Source.WorkerName,
		Type:          payload.Type,
		Severity:      severity,
		Timestamp:     timestamp,
		ProjectName:   payload.Source.WorkerName,
		ServiceName:   trigger.RepoName,
		Status:        status,
		Branch:        trigger.Branch,
		CommitHash:    trigger.CommitHash,
		CommitAuthor:  trigger.Author,
		CommitMessage: trigger.CommitMessage,
	}
}
