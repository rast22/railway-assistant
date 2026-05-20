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
