package types

import (
	"fmt"
	"time"
)

const ProviderRailway = "railway"

type NotificationEvent struct {
	Provider             string
	SourceID             string
	SourceName           string
	Type                 string
	Severity             string
	Timestamp            time.Time
	WorkspaceName        string
	ProjectName          string
	EnvironmentName      string
	EnvironmentEphemeral bool
	ServiceName          string
	Status               string
	Branch               string
	CommitHash           string
	CommitAuthor         string
	CommitMessage        string
	DeploymentURL        string
	Test                 bool
}

func RailwayAlertToNotificationEvent(payload RailwayAlert) NotificationEvent {
	event := NotificationEvent{
		Provider:             ProviderRailway,
		SourceID:             payload.Resource.Project.ID,
		SourceName:           payload.Resource.Project.Name,
		Type:                 payload.Type,
		Severity:             payload.Severity,
		Timestamp:            payload.Timestamp,
		WorkspaceName:        payload.Resource.Workspace.Name,
		ProjectName:          payload.Resource.Project.Name,
		EnvironmentName:      payload.Resource.Environment.Name,
		EnvironmentEphemeral: payload.Resource.Environment.IsEphemeral,
		Status:               payload.Details.Status,
		Branch:               payload.Details.Branch,
		CommitHash:           payload.Details.CommitHash,
		CommitAuthor:         payload.Details.CommitAuthor,
		CommitMessage:        payload.Details.CommitMessage,
	}

	if payload.Resource.Service != nil {
		event.ServiceName = payload.Resource.Service.Name
	}

	if payload.Resource.Deployment != nil && payload.Resource.Service != nil {
		event.DeploymentURL = fmt.Sprintf("https://railway.com/project/%s/service/%s?environmentId=%s&deploymentId=%s",
			payload.Resource.Project.ID,
			payload.Resource.Service.ID,
			payload.Resource.Environment.ID,
			payload.Resource.Deployment.ID,
		)
	}

	return event
}
