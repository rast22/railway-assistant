package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStorePersistsProjectsRoutesAndDestinations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "railway-assistant", "config.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if err := store.UpsertKnownProject(KnownProject{
		ID:       "project-1",
		Provider: ProviderRailway,
		Name:     "API",
	}); err != nil {
		t.Fatalf("UpsertKnownProject() error = %v", err)
	}

	_, err = store.ConnectTelegramDestination(ProviderRailway, "project-1", TelegramDestination{
		ChatID:   "-1001",
		Label:    "Deployments",
		ChatType: "supergroup",
	}, 42)
	if err != nil {
		t.Fatalf("ConnectTelegramDestination() error = %v", err)
	}

	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatalf("reload NewStore() error = %v", err)
	}

	project, ok := reloaded.KnownProject(ProviderRailway, "project-1")
	if !ok {
		t.Fatal("KnownProject() missing project")
	}
	if project.Name != "API" {
		t.Fatalf("KnownProject().Name = %q, want API", project.Name)
	}

	destinations := reloaded.TelegramDestinations(ProviderRailway, "project-1")
	if len(destinations) != 1 {
		t.Fatalf("TelegramDestinations() len = %d, want 1", len(destinations))
	}
	if destinations[0].ChatID != "-1001" {
		t.Fatalf("destination ChatID = %q, want -1001", destinations[0].ChatID)
	}
}

func TestStorePersistsTelegramTopicDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	_, err = store.ConnectTelegramDestination(ProviderRailway, "project-1", TelegramDestination{
		ChatID: "https://t.me/c/3963321501/4",
		Label:  "Deployments Topic",
	}, 42)
	if err != nil {
		t.Fatalf("ConnectTelegramDestination() error = %v", err)
	}

	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatalf("reload NewStore() error = %v", err)
	}

	destinations := reloaded.TelegramDestinations(ProviderRailway, "project-1")
	if len(destinations) != 1 {
		t.Fatalf("TelegramDestinations() len = %d, want 1", len(destinations))
	}
	if destinations[0].ChatID != "-1003963321501" {
		t.Fatalf("destination ChatID = %q, want -1003963321501", destinations[0].ChatID)
	}
	if destinations[0].MessageThreadID != 4 {
		t.Fatalf("destination MessageThreadID = %d, want 4", destinations[0].MessageThreadID)
	}
	if destinations[0].ID != "telegram:-1003963321501:thread:4" {
		t.Fatalf("destination ID = %q, want telegram:-1003963321501:thread:4", destinations[0].ID)
	}
}

func TestParseTelegramDestinationRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		chatID   string
		threadID int
		ok       bool
	}{
		{name: "plain chat", input: "-1001", chatID: "-1001", ok: true},
		{name: "private forum topic link", input: "https://t.me/c/3963321501/4", chatID: "-1003963321501", threadID: 4, ok: true},
		{name: "public chat link", input: "https://t.me/deployments", chatID: "@deployments", ok: true},
		{name: "public forum topic link", input: "https://t.me/deployments/4", chatID: "@deployments", threadID: 4, ok: true},
		{name: "plain chat and topic", input: "-1003963321501/4", chatID: "-1003963321501", threadID: 4, ok: true},
		{name: "invite link", input: "https://t.me/+abcdef", ok: false},
		{name: "blank", input: "", ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatID, threadID, ok := ParseTelegramDestinationRef(tt.input)
			if ok != tt.ok {
				t.Fatalf("ok = %t, want %t", ok, tt.ok)
			}
			if chatID != tt.chatID {
				t.Fatalf("chatID = %q, want %q", chatID, tt.chatID)
			}
			if threadID != tt.threadID {
				t.Fatalf("threadID = %d, want %d", threadID, tt.threadID)
			}
		})
	}
}

func TestStoreDisconnectRemovesRoute(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	_, err = store.ConnectTelegramDestination(ProviderRailway, "project-1", TelegramDestination{ChatID: "-1001"}, 42)
	if err != nil {
		t.Fatalf("ConnectTelegramDestination() error = %v", err)
	}

	removed, err := store.DisconnectTelegramDestination(ProviderRailway, "project-1", "")
	if err != nil {
		t.Fatalf("DisconnectTelegramDestination() error = %v", err)
	}
	if !removed {
		t.Fatal("DisconnectTelegramDestination() removed = false, want true")
	}

	if got := store.TelegramDestinations(ProviderRailway, "project-1"); len(got) != 0 {
		t.Fatalf("TelegramDestinations() len = %d, want 0", len(got))
	}
}

func TestStoreKeepsKnownProjectsSeparateByProvider(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if err := store.UpsertKnownProject(KnownProject{ID: "api", Provider: ProviderRailway, Name: "Railway API"}); err != nil {
		t.Fatalf("UpsertKnownProject(railway) error = %v", err)
	}
	if err := store.UpsertKnownProject(KnownProject{ID: "api", Provider: ProviderCloudflare, Name: "Cloudflare API"}); err != nil {
		t.Fatalf("UpsertKnownProject(cloudflare) error = %v", err)
	}

	railway, ok := store.KnownProject(ProviderRailway, "api")
	if !ok || railway.Name != "Railway API" {
		t.Fatalf("railway project = %#v, ok=%t", railway, ok)
	}
	cloudflare, ok := store.KnownProject(ProviderCloudflare, "api")
	if !ok || cloudflare.Name != "Cloudflare API" {
		t.Fatalf("cloudflare project = %#v, ok=%t", cloudflare, ok)
	}
}

func TestStoreRejectsMalformedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := NewStore(path); err == nil {
		t.Fatal("NewStore() error = nil, want malformed JSON error")
	}
}

func TestNewStoreFromEnvRequiresDurablePathForBotManagement(t *testing.T) {
	t.Setenv("ADMIN_TELEGRAM_USER_IDS", "42")
	t.Setenv("CONFIG_PATH", "")
	t.Setenv("RAILWAY_VOLUME_MOUNT_PATH", "")

	_, err := NewStoreFromEnv()
	if !errors.Is(err, ErrNoDurablePath) {
		t.Fatalf("NewStoreFromEnv() error = %v, want ErrNoDurablePath", err)
	}
}
