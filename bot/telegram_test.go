package bot

import (
	"path/filepath"
	"strings"
	"testing"

	"railway-assistant/config"
	"railway-assistant/notifications"
	"railway-assistant/services"
)

type botTelegramSender struct {
	messages  []botTelegramMessage
	callbacks []string
}

type botTelegramMessage struct {
	chatID  string
	message services.TelegramMessage
}

func (s *botTelegramSender) SendMessage(chatID string, message services.TelegramMessage) error {
	s.messages = append(s.messages, botTelegramMessage{chatID: chatID, message: message})
	return nil
}

func (s *botTelegramSender) AnswerCallbackQuery(callbackID, text string) error {
	s.callbacks = append(s.callbacks, callbackID+":"+text)
	return nil
}

func TestBotRejectsUnauthorizedUser(t *testing.T) {
	store := newBotTestStore(t)
	sender := &botTelegramSender{}
	handler := Handler{
		Store:    store,
		Telegram: sender,
		Admins:   map[int64]bool{42: true},
	}

	err := handler.HandleUpdate(Update{Message: &Message{
		From: &User{ID: 7},
		Chat: Chat{ID: 100, Type: "private"},
		Text: "/status",
	}})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v", err)
	}
	if len(sender.messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(sender.messages))
	}
	if !strings.Contains(sender.messages[0].message.Text, "Unauthorized") {
		t.Fatalf("message text = %q, want unauthorized", sender.messages[0].message.Text)
	}
}

func TestBotProjectsConnectRoutesDisconnectAndTest(t *testing.T) {
	store := newBotTestStore(t)
	if err := store.UpsertKnownProject(config.KnownProject{
		ID:       "project-1",
		Provider: config.ProviderRailway,
		Name:     "API",
	}); err != nil {
		t.Fatalf("UpsertKnownProject() error = %v", err)
	}

	sender := &botTelegramSender{}
	dispatcher := notifications.NewDispatcher(store, sender)
	handler := Handler{
		Store:      store,
		Telegram:   sender,
		Dispatcher: dispatcher,
		Admins:     map[int64]bool{42: true},
	}

	updates := []Update{
		{Message: &Message{From: &User{ID: 42}, Chat: Chat{ID: 100, Type: "private"}, Text: "/projects"}},
		{Message: &Message{From: &User{ID: 42}, Chat: Chat{ID: 100, Type: "private"}, Text: "/connect project-1 -1001 Deployments"}},
		{Message: &Message{From: &User{ID: 42}, Chat: Chat{ID: 100, Type: "private"}, Text: "/routes"}},
		{Message: &Message{From: &User{ID: 42}, Chat: Chat{ID: 100, Type: "private"}, Text: "/test project-1"}},
		{Message: &Message{From: &User{ID: 42}, Chat: Chat{ID: 100, Type: "private"}, Text: "/disconnect project-1 -1001"}},
	}

	for _, update := range updates {
		if err := handler.HandleUpdate(update); err != nil {
			t.Fatalf("HandleUpdate(%q) error = %v", update.Message.Text, err)
		}
	}

	if len(sender.messages) < 6 {
		t.Fatalf("messages len = %d, want at least 6", len(sender.messages))
	}
	if !strings.Contains(sender.messages[0].message.Text, "Known Railway projects") {
		t.Fatalf("/projects text = %q", sender.messages[0].message.Text)
	}
	if sender.messages[0].message.ReplyMarkup == nil {
		t.Fatal("/projects did not include inline keyboard")
	}

	destinations := store.TelegramDestinations(config.ProviderRailway, "project-1")
	if len(destinations) != 0 {
		t.Fatalf("destinations len after disconnect = %d, want 0", len(destinations))
	}

	var sentTest bool
	for _, message := range sender.messages {
		if message.chatID == "-1001" && strings.Contains(message.message.Text, "Telegram route test") {
			sentTest = true
		}
	}
	if !sentTest {
		t.Fatal("test notification was not sent to connected destination")
	}
}

func TestBotPrivateConnectWithExplicitChatID(t *testing.T) {
	store := newBotTestStore(t)
	sender := &botTelegramSender{}
	handler := Handler{
		Store:    store,
		Telegram: sender,
		Admins:   map[int64]bool{42: true},
	}

	err := handler.HandleUpdate(Update{Message: &Message{
		From: &User{ID: 42},
		Chat: Chat{ID: 100, Type: "private"},
		Text: "/connect project-1 -1001",
	}})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v", err)
	}

	destinations := store.TelegramDestinations(config.ProviderRailway, "project-1")
	if len(destinations) != 1 || destinations[0].ChatID != "-1001" {
		t.Fatalf("destinations = %#v, want chat -1001", destinations)
	}
}

func TestBotConnectRequiresExplicitChatID(t *testing.T) {
	store := newBotTestStore(t)
	sender := &botTelegramSender{}
	handler := Handler{
		Store:    store,
		Telegram: sender,
		Admins:   map[int64]bool{42: true},
	}

	err := handler.HandleUpdate(Update{Message: &Message{
		From: &User{ID: 42},
		Chat: Chat{ID: -1001, Type: "supergroup", Title: "Deployments"},
		Text: "/connect project-1",
	}})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v", err)
	}

	if destinations := store.TelegramDestinations(config.ProviderRailway, "project-1"); len(destinations) != 0 {
		t.Fatalf("destinations = %#v, want none", destinations)
	}
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0].message.Text, "/connect <project_id> <chat_id>") {
		t.Fatalf("usage reply = %#v", sender.messages)
	}
}

func newBotTestStore(t *testing.T) *config.Store {
	t.Helper()
	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}
