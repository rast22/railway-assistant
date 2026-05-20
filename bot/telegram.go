package bot

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"railway-assistant/config"
	"railway-assistant/env"
	"railway-assistant/notifications"
	"railway-assistant/services"
	"railway-assistant/types"
)

type Handler struct {
	Store      *config.Store
	Telegram   services.TelegramSender
	Dispatcher *notifications.Dispatcher
	Admins     map[int64]bool
}

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text,omitempty"`
}

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

func NewHandler(store *config.Store, telegram services.TelegramSender, dispatcher *notifications.Dispatcher) Handler {
	return Handler{
		Store:      store,
		Telegram:   telegram,
		Dispatcher: dispatcher,
		Admins:     AdminIDsFromEnv(),
	}
}

func AdminIDsFromEnv() map[int64]bool {
	admins := map[int64]bool{}
	for _, raw := range strings.Split(env.GetString("ADMIN_TELEGRAM_USER_IDS", ""), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			log.Printf("invalid ADMIN_TELEGRAM_USER_IDS entry %q: %v", raw, err)
			continue
		}
		admins[id] = true
	}
	return admins
}

func (h Handler) HandleUpdate(update Update) error {
	if update.CallbackQuery != nil {
		return h.handleCallback(*update.CallbackQuery)
	}
	if update.Message != nil {
		return h.handleMessage(*update.Message)
	}
	return nil
}

func (h Handler) handleMessage(message Message) error {
	if message.From == nil {
		return nil
	}
	chatID := chatIDString(message.Chat.ID)
	if !h.isAdmin(message.From.ID) {
		return h.reply(chatID, "Unauthorized.")
	}

	command, args, ok := parseCommand(message.Text)
	if !ok {
		return nil
	}

	switch command {
	case "start", "help":
		return h.reply(chatID, helpText())
	case "status":
		return h.sendStatus(chatID)
	case "projects":
		return h.sendProjects(chatID)
	case "routes":
		return h.sendRoutes(chatID)
	case "connect":
		return h.connect(chatID, message.Chat, message.From.ID, args)
	case "disconnect":
		return h.disconnect(chatID, message.Chat, args)
	case "test":
		return h.testRoute(chatID, args)
	default:
		return h.reply(chatID, "Unknown command. Send /help.")
	}
}

func (h Handler) handleCallback(callback CallbackQuery) error {
	if callback.Message == nil {
		return nil
	}
	chatID := chatIDString(callback.Message.Chat.ID)
	if !h.isAdmin(callback.From.ID) {
		_ = h.answerCallback(callback.ID, "Unauthorized")
		return h.reply(chatID, "Unauthorized.")
	}

	parts := strings.Split(callback.Data, "|")
	if len(parts) != 3 {
		_ = h.answerCallback(callback.ID, "Unsupported action")
		return nil
	}

	action, provider, sourceID := parts[0], parts[1], parts[2]
	switch action {
	case "test":
		_ = h.answerCallback(callback.ID, "Sending test")
		return h.dispatchTest(chatID, provider, sourceID)
	case "routes":
		_ = h.answerCallback(callback.ID, "Showing routes")
		return h.sendRoutes(chatID)
	default:
		_ = h.answerCallback(callback.ID, "Unsupported action")
		return nil
	}
}

func (h Handler) sendStatus(chatID string) error {
	snap, err := h.Store.Snapshot()
	if err != nil {
		return err
	}

	text := fmt.Sprintf(
		"Railway Assistant status\n\nConfig: %s\nKnown projects: %d\nRoutes: %d\nTelegram destinations: %d",
		snap.Path,
		len(snap.KnownProjects),
		len(snap.Routes),
		len(snap.TelegramDestinations),
	)
	return h.reply(chatID, text)
}

func (h Handler) sendProjects(chatID string) error {
	snap, err := h.Store.Snapshot()
	if err != nil {
		return err
	}
	if len(snap.KnownProjects) == 0 {
		return h.reply(chatID, "No Railway projects have been seen yet. Send a Railway test webhook first, then run /projects again.")
	}

	var sb strings.Builder
	sb.WriteString("Known Railway projects\n\n")
	for _, project := range snap.KnownProjects {
		sb.WriteString(fmt.Sprintf("%s\nID: %s\n\n", project.Name, project.ID))
	}

	keyboard := make([][]map[string]string, 0, len(snap.KnownProjects))
	for _, project := range snap.KnownProjects {
		label := project.Name
		if label == "" {
			label = project.ID
		}
		keyboard = append(keyboard, []map[string]string{
			{"text": "Test " + truncate(label, 24), "callback_data": "test|" + project.Provider + "|" + project.ID},
			{"text": "Routes", "callback_data": "routes|" + project.Provider + "|" + project.ID},
		})
	}

	return h.send(chatID, services.TelegramMessage{
		Text:      strings.TrimSpace(sb.String()),
		ParseMode: "none",
		ReplyMarkup: map[string]interface{}{
			"inline_keyboard": keyboard,
		},
	})
}

func (h Handler) sendRoutes(chatID string) error {
	snap, err := h.Store.Snapshot()
	if err != nil {
		return err
	}
	if len(snap.Routes) == 0 {
		return h.reply(chatID, "No routes are configured yet.")
	}

	projectNames := map[string]string{}
	for _, project := range snap.KnownProjects {
		projectNames[project.ID] = project.Name
	}
	destinationLabels := map[string]string{}
	for _, destination := range snap.TelegramDestinations {
		destinationLabels[destination.ID] = fmt.Sprintf("%s (%s)", destination.Label, destination.ChatID)
	}

	var sb strings.Builder
	sb.WriteString("Configured routes\n\n")
	for _, route := range snap.Routes {
		name := projectNames[route.SourceID]
		if name == "" {
			name = route.SourceID
		}
		sb.WriteString(fmt.Sprintf("%s\nProvider: %s\nEnabled: %t\n", name, route.Provider, route.Enabled))
		if len(route.TelegramDestination) == 0 {
			sb.WriteString("Destinations: none\n\n")
			continue
		}
		labels := make([]string, 0, len(route.TelegramDestination))
		for _, id := range route.TelegramDestination {
			label := destinationLabels[id]
			if label == "" {
				label = id
			}
			labels = append(labels, label)
		}
		sort.Strings(labels)
		sb.WriteString("Destinations:\n")
		for _, label := range labels {
			sb.WriteString("- " + label + "\n")
		}
		sb.WriteString("\n")
	}

	return h.reply(chatID, strings.TrimSpace(sb.String()))
}

func (h Handler) connect(replyChatID string, chat Chat, adminID int64, args []string) error {
	if len(args) < 2 {
		return h.reply(replyChatID, "Usage: /connect <project_id> <chat_id> [label]. Run this from your private admin chat with the bot.")
	}

	sourceID := args[0]
	destinationChatID := args[1]
	destinationLabel := "Chat " + destinationChatID
	if len(args) > 2 {
		destinationLabel = strings.Join(args[2:], " ")
	}

	destination := config.TelegramDestination{
		ChatID:           destinationChatID,
		Label:            destinationLabel,
		ChatType:         "manual",
		CreatedByAdminID: adminID,
	}
	if _, err := h.Store.ConnectTelegramDestination(config.ProviderRailway, sourceID, destination, adminID); err != nil {
		return err
	}

	projectName := sourceID
	if project, ok := h.Store.KnownProject(config.ProviderRailway, sourceID); ok && project.Name != "" {
		projectName = project.Name
	}

	return h.reply(replyChatID, fmt.Sprintf("Connected %s to %s.", projectName, destinationLabel))
}

func (h Handler) disconnect(replyChatID string, chat Chat, args []string) error {
	if len(args) < 2 {
		return h.reply(replyChatID, "Usage: /disconnect <project_id> <chat_id>.")
	}

	sourceID := args[0]
	chatID := args[1]

	removed, err := h.Store.DisconnectTelegramDestination(config.ProviderRailway, sourceID, chatID)
	if err != nil {
		return err
	}
	if !removed {
		return h.reply(replyChatID, "No matching route was found.")
	}
	return h.reply(replyChatID, "Route updated.")
}

func (h Handler) testRoute(chatID string, args []string) error {
	if len(args) == 0 {
		return h.reply(chatID, "Usage: /test <project_id>.")
	}
	return h.dispatchTest(chatID, config.ProviderRailway, args[0])
}

func (h Handler) dispatchTest(replyChatID, provider, sourceID string) error {
	if h.Dispatcher == nil {
		return h.reply(replyChatID, "Notification dispatcher is not configured.")
	}

	projectName := sourceID
	if project, ok := h.Store.KnownProject(provider, sourceID); ok && project.Name != "" {
		projectName = project.Name
	}

	event := types.NotificationEvent{
		Provider:        provider,
		SourceID:        sourceID,
		SourceName:      projectName,
		Type:            "test",
		Severity:        "info",
		Timestamp:       time.Now().UTC(),
		ProjectName:     projectName,
		EnvironmentName: "Test",
		Status:          "success",
		CommitMessage:   "Telegram route test",
		Test:            true,
	}
	result := h.Dispatcher.DispatchTelegram(event)
	if len(result.Errors) > 0 {
		return h.reply(replyChatID, fmt.Sprintf("Test attempted, but %d send error(s) occurred.", len(result.Errors)))
	}
	if result.Sent == 0 {
		return h.reply(replyChatID, "No Telegram destinations matched this project.")
	}
	return h.reply(replyChatID, fmt.Sprintf("Sent test notification to %d destination(s).", result.Sent))
}

func (h Handler) reply(chatID, text string) error {
	return h.send(chatID, services.TelegramMessage{
		Text:      text,
		ParseMode: "none",
	})
}

func (h Handler) send(chatID string, message services.TelegramMessage) error {
	if h.Telegram == nil {
		return nil
	}
	return h.Telegram.SendMessage(chatID, message)
}

func (h Handler) answerCallback(callbackID, text string) error {
	if h.Telegram == nil {
		return nil
	}
	return h.Telegram.AnswerCallbackQuery(callbackID, text)
}

func (h Handler) isAdmin(userID int64) bool {
	return len(h.Admins) > 0 && h.Admins[userID]
}

func parseCommand(text string) (string, []string, bool) {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "/") {
		return "", nil, false
	}

	command := strings.TrimPrefix(fields[0], "/")
	if at := strings.Index(command, "@"); at >= 0 {
		command = command[:at]
	}
	command = strings.ToLower(command)
	return command, fields[1:], true
}

func helpText() string {
	return strings.TrimSpace(`
Railway Assistant commands

/status - show configuration status
/projects - list Railway projects seen from webhooks
/routes - list configured routes
/connect <project_id> <chat_id> [label] - connect a project to a Telegram chat
/disconnect <project_id> <chat_id> - remove a Telegram chat from a project
/test <project_id> - send a test notification
`)
}

func chatIDString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func chatLabel(chat Chat) string {
	switch {
	case chat.Title != "":
		return chat.Title
	case chat.Username != "":
		return "@" + chat.Username
	case strings.TrimSpace(chat.FirstName+" "+chat.LastName) != "":
		return strings.TrimSpace(chat.FirstName + " " + chat.LastName)
	default:
		return chatIDString(chat.ID)
	}
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	if max <= 1 {
		return value[:max]
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}
