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

func TestBotIgnoresUnauthorizedUser(t *testing.T) {
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
	if len(sender.messages) != 0 {
		t.Fatalf("messages len = %d, want 0", len(sender.messages))
	}
	if len(sender.callbacks) != 0 {
		t.Fatalf("callbacks len = %d, want 0", len(sender.callbacks))
	}
}

func TestBotIgnoresUnauthorizedCallback(t *testing.T) {
	store := newBotTestStore(t)
	sender := &botTelegramSender{}
	handler := Handler{
		Store:    store,
		Telegram: sender,
		Admins:   map[int64]bool{42: true},
	}

	err := handler.HandleUpdate(Update{CallbackQuery: &CallbackQuery{
		ID:   "callback-1",
		From: User{ID: 7},
		Message: &Message{
			Chat: Chat{ID: 100, Type: "private"},
		},
		Data: "routes|railway|project-1",
	}})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v", err)
	}
	if len(sender.messages) != 0 {
		t.Fatalf("messages len = %d, want 0", len(sender.messages))
	}
	if len(sender.callbacks) != 0 {
		t.Fatalf("callbacks len = %d, want 0", len(sender.callbacks))
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
	if !strings.Contains(sender.messages[0].message.Text, "Known sources") || !strings.Contains(sender.messages[0].message.Text, "Railway") {
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

func TestBotConnectsTelegramTopicLink(t *testing.T) {
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
		Text: "/connect project-1 https://t.me/c/3963321501/4 Deploy Topic",
	}})
	if err != nil {
		t.Fatalf("HandleUpdate() error = %v", err)
	}

	destinations := store.TelegramDestinations(config.ProviderRailway, "project-1")
	if len(destinations) != 1 {
		t.Fatalf("destinations len = %d, want 1", len(destinations))
	}
	if destinations[0].ChatID != "-1003963321501" {
		t.Fatalf("ChatID = %q, want -1003963321501", destinations[0].ChatID)
	}
	if destinations[0].MessageThreadID != 4 {
		t.Fatalf("MessageThreadID = %d, want 4", destinations[0].MessageThreadID)
	}
	if destinations[0].Label != "Deploy Topic" {
		t.Fatalf("Label = %q, want Deploy Topic", destinations[0].Label)
	}
}

func TestBotCloudflareProjectsConnectAndTest(t *testing.T) {
	store := newBotTestStore(t)
	if err := store.UpsertKnownProject(config.KnownProject{
		ID:       "my-worker",
		Provider: config.ProviderCloudflare,
		Name:     "my-worker",
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
		{Message: &Message{From: &User{ID: 42}, Chat: Chat{ID: 100, Type: "private"}, Text: "/connect cf my-worker -2001 Worker Builds"}},
		{Message: &Message{From: &User{ID: 42}, Chat: Chat{ID: 100, Type: "private"}, Text: "/test cf my-worker"}},
	}

	for _, update := range updates {
		if err := handler.HandleUpdate(update); err != nil {
			t.Fatalf("HandleUpdate(%q) error = %v", update.Message.Text, err)
		}
	}

	if !strings.Contains(sender.messages[0].message.Text, "Cloudflare Workers") {
		t.Fatalf("/projects text = %q, want Cloudflare Workers group", sender.messages[0].message.Text)
	}

	destinations := store.TelegramDestinations(config.ProviderCloudflare, "my-worker")
	if len(destinations) != 1 || destinations[0].ChatID != "-2001" {
		t.Fatalf("destinations = %#v, want chat -2001", destinations)
	}

	var sentTest bool
	for _, message := range sender.messages {
		if message.chatID == "-2001" &&
			strings.Contains(message.message.Text, "Cloudflare Workers Build") &&
			strings.Contains(message.message.Text, "Cloudflare Workers route test") {
			sentTest = true
		}
	}
	if !sentTest {
		t.Fatal("cloudflare test notification was not sent to connected destination")
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
	if len(sender.messages) != 1 || !strings.Contains(sender.messages[0].message.Text, "/connect <project_id> <chat_id_or_topic_link>") {
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
