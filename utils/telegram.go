package utils

import (
	"fmt"
	"railway-assistant/types"
	"strings"
)

func PrepareTelegramMessage(payload types.RailwayAlert) (string, string) {
	return PrepareTelegramNotificationMessage(types.RailwayAlertToNotificationEvent(payload))
}

func PrepareTelegramNotificationMessage(event types.NotificationEvent) (string, string) {
	var sb strings.Builder
	emoji := GetNotificationStatusEmoji(event)

	sb.WriteString("🚄 *Railway Alert*\n\n")

	eventType := strings.ToUpper(FormatEventType(event.Type))
	sb.WriteString(fmt.Sprintf("%s *%s*\n\n", emoji, EscapeMarkdown(eventType)))

	sb.WriteString("*Details*\n")
	sb.WriteString("📍 ")
	pathParts := []string{}

	if IsWorkspaceIncluded() && event.WorkspaceName != "" {
		pathParts = append(pathParts, EscapeMarkdown(event.WorkspaceName))
	}

	projectName := event.ProjectName
	if projectName == "" {
		projectName = event.SourceName
	}
	pathParts = append(pathParts, EscapeMarkdown(projectName))

	if event.ServiceName != "" {
		pathParts = append(pathParts, EscapeMarkdown(event.ServiceName))
	}

	sb.WriteString(strings.Join(pathParts, " / "))

	if event.EnvironmentName != "" {
		sb.WriteString(fmt.Sprintf("\n🌍 %s", EscapeMarkdown(event.EnvironmentName)))
	}
	if event.EnvironmentEphemeral {
		sb.WriteString(" \\(Ephemeral\\)")
	}

	if IsStatusIncluded() && event.Status != "" {
		sb.WriteString(fmt.Sprintf("\n🔄 Status: %s", EscapeMarkdown(TitleCase(event.Status))))
	}
	sb.WriteString("\n\n")

	var gitSb strings.Builder
	hasGitInfo := false

	if IsCommitIncluded() && event.CommitMessage != "" {
		gitSb.WriteString(fmt.Sprintf("💬 _%s_\n", EscapeMarkdown(event.CommitMessage)))
		hasGitInfo = true
	}

	metaParts := []string{}

	if IsBranchIncluded() && event.Branch != "" {
		metaParts = append(metaParts, fmt.Sprintf("🌱 %s", EscapeMarkdown(event.Branch)))
	}

	if IsAuthorIncluded() && event.CommitAuthor != "" {
		metaParts = append(metaParts, fmt.Sprintf("👤 %s", EscapeMarkdown(event.CommitAuthor)))
	}

	if len(metaParts) > 0 {
		gitSb.WriteString(strings.Join(metaParts, "  •  "))
		gitSb.WriteString("\n")
		hasGitInfo = true
	}

	if hasGitInfo {
		sb.WriteString("*Git*\n")
		sb.WriteString(gitSb.String())
	}

	if event.Test {
		sb.WriteString("\n_Test notification_")
	}

	return sb.String(), event.DeploymentURL
}

func EscapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
		"\\", "\\\\",
	)
	return replacer.Replace(text)
}
