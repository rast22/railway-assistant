package utils

import (
	"railway-assistant/types"
	"strings"
)

func GetStatusEmoji(payload types.RailwayAlert) string {
	return GetNotificationStatusEmoji(types.RailwayAlertToNotificationEvent(payload))
}

func GetNotificationStatusEmoji(event types.NotificationEvent) string {
	status := strings.ToUpper(event.Status)
	severity := strings.ToUpper(event.Severity)

	if status == "SUCCESS" || status == "SUCCEEDED" {
		return "🟢"
	}
	if status == "FAILED" || status == "FAILURE" {
		return "🔴"
	}
	if status == "CANCELLED" || status == "CANCELED" {
		return "🟠"
	}

	switch severity {
	case "ERROR":
		return "🔴"
	case "WARNING", "WARN":
		return "🟠"
	case "INFO":
		return "🔵"
	}

	eventType := strings.ToLower(event.Type)
	if strings.Contains(eventType, "fail") || strings.Contains(eventType, "error") || strings.Contains(eventType, "crash") {
		return "🔴"
	}
	if strings.Contains(eventType, "success") {
		return "🟢"
	}

	return "🔵"
}

func FormatEventType(t string) string {
	upper := strings.ToUpper(t)
	text := strings.ReplaceAll(upper, "_", " ")
	text = strings.ReplaceAll(text, ".", " ")

	if strings.HasPrefix(upper, "DEPLOYMENT") || strings.HasPrefix(upper, "DEPLOY") {
		if strings.Contains(upper, "REDEPLOY") {
			return "Redeployed"
		}
		if strings.Contains(upper, "DEPLOYED") {
			return "Deployed"
		}
		if strings.Contains(upper, "BUILDING") {
			return "Building"
		}
		if strings.Contains(upper, "DEPLOYING") {
			return "Deploying"
		}
		if upper == "DEPLOY" {
			return "Deployed"
		}
	}

	return TitleCase(text)
}

func TitleCase(s string) string {
	s = strings.ToLower(s)
	parts := strings.Fields(s)
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
