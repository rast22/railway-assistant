package notifications

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"railway-assistant/config"
	"railway-assistant/services"
	"railway-assistant/types"
)

type fakeTelegramSender struct {
	mu       sync.Mutex
	messages []sentTelegramMessage
}

type sentTelegramMessage struct {
	chatID  string
	message services.TelegramMessage
}

func (f *fakeTelegramSender) SendMessage(chatID string, message services.TelegramMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, sentTelegramMessage{chatID: chatID, message: message})
	return nil
}

func (f *fakeTelegramSender) AnswerCallbackQuery(callbackID, text string) error {
	return nil
}

func (f *fakeTelegramSender) chats() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	chats := make([]string, 0, len(f.messages))
	for _, message := range f.messages {
		chats = append(chats, message.chatID)
	}
	return chats
}

func TestDispatcherSendsToAllMatchingTelegramDestinations(t *testing.T) {
	t.Setenv("TELEGRAM_CHAT_ID", "")
	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if _, err := store.ConnectTelegramDestination(config.ProviderRailway, "project-1", config.TelegramDestination{ChatID: "-1001"}, 42); err != nil {
		t.Fatalf("ConnectTelegramDestination() error = %v", err)
	}
	if _, err := store.ConnectTelegramDestination(config.ProviderRailway, "project-1", config.TelegramDestination{ChatID: "-1002"}, 42); err != nil {
		t.Fatalf("ConnectTelegramDestination() error = %v", err)
	}

	sender := &fakeTelegramSender{}
	result := NewDispatcher(store, sender).DispatchTelegram(testEvent("project-1"))

	if result.MatchedDestinations != 2 {
		t.Fatalf("MatchedDestinations = %d, want 2", result.MatchedDestinations)
	}
	if result.Sent != 2 {
		t.Fatalf("Sent = %d, want 2", result.Sent)
	}
	if got := sender.chats(); len(got) != 2 || got[0] != "-1001" || got[1] != "-1002" {
		t.Fatalf("sent chats = %#v, want [-1001 -1002]", got)
	}
}

func TestDispatcherUsesLegacyFallbackWhenNoRouteMatches(t *testing.T) {
	t.Setenv("TELEGRAM_CHAT_ID", "-999")
	store, err := config.NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	sender := &fakeTelegramSender{}
	result := NewDispatcher(store, sender).DispatchTelegram(testEvent("project-1"))

	if !result.UsedLegacyFallback {
		t.Fatal("UsedLegacyFallback = false, want true")
	}
	if result.Sent != 1 {
		t.Fatalf("Sent = %d, want 1", result.Sent)
	}
	if got := sender.chats(); len(got) != 1 || got[0] != "-999" {
		t.Fatalf("sent chats = %#v, want [-999]", got)
	}
}

func TestDispatcherSkipsDisabledRoutesAndDestinations(t *testing.T) {
	t.Setenv("TELEGRAM_CHAT_ID", "")
	path := filepath.Join(t.TempDir(), "config.json")
	data := config.Data{
		SchemaVersion: config.SchemaVersion,
		Routes: map[string]config.Route{
			config.RouteID(config.ProviderRailway, "disabled-route"): {
				ID:                  config.RouteID(config.ProviderRailway, "disabled-route"),
				Provider:            config.ProviderRailway,
				SourceID:            "disabled-route",
				Enabled:             false,
				TelegramDestination: []string{config.TelegramDestinationID("-1001")},
			},
			config.RouteID(config.ProviderRailway, "disabled-destination"): {
				ID:                  config.RouteID(config.ProviderRailway, "disabled-destination"),
				Provider:            config.ProviderRailway,
				SourceID:            "disabled-destination",
				Enabled:             true,
				TelegramDestination: []string{config.TelegramDestinationID("-1002")},
			},
		},
		TelegramDestinations: map[string]config.TelegramDestination{
			config.TelegramDestinationID("-1001"): {ID: config.TelegramDestinationID("-1001"), ChatID: "-1001", Enabled: true},
			config.TelegramDestinationID("-1002"): {ID: config.TelegramDestinationID("-1002"), ChatID: "-1002", Enabled: false},
		},
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := writeTestFile(path, bytes); err != nil {
		t.Fatalf("writeTestFile() error = %v", err)
	}

	store, err := config.NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	sender := &fakeTelegramSender{}
	dispatcher := NewDispatcher(store, sender)

	if result := dispatcher.DispatchTelegram(testEvent("disabled-route")); result.Sent != 0 {
		t.Fatalf("disabled route Sent = %d, want 0", result.Sent)
	}
	if result := dispatcher.DispatchTelegram(testEvent("disabled-destination")); result.Sent != 0 {
		t.Fatalf("disabled destination Sent = %d, want 0", result.Sent)
	}
	if got := sender.chats(); len(got) != 0 {
		t.Fatalf("sent chats = %#v, want none", got)
	}
}

func testEvent(sourceID string) types.NotificationEvent {
	return types.NotificationEvent{
		Provider:        types.ProviderRailway,
		SourceID:        sourceID,
		SourceName:      "API",
		Type:            "deployment_success",
		Severity:        "info",
		Timestamp:       time.Now(),
		ProjectName:     "API",
		EnvironmentName: "Production",
		Status:          "success",
	}
}

func writeTestFile(path string, bytes []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0o644)
}
