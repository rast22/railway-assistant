package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"railway-assistant/env"
)

const (
	SchemaVersion = 1

	ProviderRailway  = "railway"
	ProviderTelegram = "telegram"
)

var (
	ErrNoDurablePath = errors.New("attach a Railway Volume or set CONFIG_PATH for bot-managed routing config")
	ErrNotReady      = errors.New("config store is not ready")
)

type Store struct {
	mu   sync.Mutex
	path string
	data Data
}

type Data struct {
	SchemaVersion        int                            `json:"schema_version"`
	KnownProjects        map[string]KnownProject        `json:"known_projects"`
	Routes               map[string]Route               `json:"routes"`
	TelegramDestinations map[string]TelegramDestination `json:"telegram_destinations"`
	CreatedAt            time.Time                      `json:"created_at"`
	UpdatedAt            time.Time                      `json:"updated_at"`
}

type KnownProject struct {
	ID         string    `json:"id"`
	Provider   string    `json:"provider"`
	Name       string    `json:"name"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type Route struct {
	ID                  string    `json:"id"`
	Provider            string    `json:"provider"`
	SourceID            string    `json:"source_id"`
	Enabled             bool      `json:"enabled"`
	TelegramDestination []string  `json:"telegram_destination_ids"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	CreatedByAdminID    int64     `json:"created_by_admin_id"`
}

type TelegramDestination struct {
	ID               string    `json:"id"`
	ChatID           string    `json:"chat_id"`
	Label            string    `json:"label"`
	ChatType         string    `json:"chat_type"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedByAdminID int64     `json:"created_by_admin_id"`
}

type Snapshot struct {
	Path                 string
	SchemaVersion        int
	KnownProjects        []KnownProject
	Routes               []Route
	TelegramDestinations []TelegramDestination
}

func ManagedConfigEnabledFromEnv() bool {
	return strings.TrimSpace(env.GetString("ADMIN_TELEGRAM_USER_IDS", "")) != "" ||
		strings.TrimSpace(env.GetString("TELEGRAM_WEBHOOK_SECRET", "")) != "" ||
		strings.TrimSpace(env.GetString("PUBLIC_BASE_URL", "")) != ""
}

func ResolvePathFromEnv() (string, bool) {
	if explicit := strings.TrimSpace(env.GetString("CONFIG_PATH", "")); explicit != "" {
		return explicit, true
	}

	if mount := strings.TrimSpace(env.GetString("RAILWAY_VOLUME_MOUNT_PATH", "")); mount != "" {
		return filepath.Join(mount, "railway-assistant", "config.json"), true
	}

	return "", false
}

func NewStoreFromEnv() (*Store, error) {
	path, ok := ResolvePathFromEnv()
	required := ManagedConfigEnabledFromEnv()
	if !ok {
		if required {
			return nil, ErrNoDurablePath
		}
		return nil, nil
	}

	return NewStore(path)
}

func NewStore(path string) (*Store, error) {
	store := &Store{path: path}
	if err := store.load(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

func (s *Store) Ready() bool {
	return s != nil && s.path != ""
}

func (s *Store) Snapshot() (Snapshot, error) {
	if s == nil {
		return Snapshot{}, ErrNotReady
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	snap := Snapshot{
		Path:                 s.path,
		SchemaVersion:        s.data.SchemaVersion,
		KnownProjects:        make([]KnownProject, 0, len(s.data.KnownProjects)),
		Routes:               make([]Route, 0, len(s.data.Routes)),
		TelegramDestinations: make([]TelegramDestination, 0, len(s.data.TelegramDestinations)),
	}

	for _, project := range s.data.KnownProjects {
		snap.KnownProjects = append(snap.KnownProjects, project)
	}
	sort.Slice(snap.KnownProjects, func(i, j int) bool {
		return snap.KnownProjects[i].Name < snap.KnownProjects[j].Name
	})

	for _, route := range s.data.Routes {
		snap.Routes = append(snap.Routes, route)
	}
	sort.Slice(snap.Routes, func(i, j int) bool {
		return snap.Routes[i].SourceID < snap.Routes[j].SourceID
	})

	for _, destination := range s.data.TelegramDestinations {
		snap.TelegramDestinations = append(snap.TelegramDestinations, destination)
	}
	sort.Slice(snap.TelegramDestinations, func(i, j int) bool {
		return snap.TelegramDestinations[i].Label < snap.TelegramDestinations[j].Label
	})

	return snap, nil
}

func (s *Store) UpsertKnownProject(project KnownProject) error {
	if s == nil {
		return nil
	}

	project.ID = strings.TrimSpace(project.ID)
	if project.ID == "" {
		return nil
	}
	if project.Provider == "" {
		project.Provider = ProviderRailway
	}
	if project.LastSeenAt.IsZero() {
		project.LastSeenAt = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.data.KnownProjects[project.ID]
	if exists {
		if project.Name == "" {
			project.Name = existing.Name
		}
		if project.Provider == "" {
			project.Provider = existing.Provider
		}
	}
	s.data.KnownProjects[project.ID] = project
	return s.persistLocked()
}

func (s *Store) KnownProject(provider, sourceID string) (KnownProject, bool) {
	if s == nil {
		return KnownProject{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	project, ok := s.data.KnownProjects[sourceID]
	if !ok || project.Provider != provider {
		return KnownProject{}, false
	}
	return project, true
}

func (s *Store) ConnectTelegramDestination(provider, sourceID string, destination TelegramDestination, adminID int64) (Route, error) {
	if s == nil {
		return Route{}, ErrNotReady
	}

	sourceID = strings.TrimSpace(sourceID)
	destination.ChatID = strings.TrimSpace(destination.ChatID)
	if provider == "" {
		provider = ProviderRailway
	}
	if sourceID == "" {
		return Route{}, fmt.Errorf("source id is required")
	}
	if destination.ChatID == "" {
		return Route{}, fmt.Errorf("telegram chat id is required")
	}

	now := time.Now().UTC()
	if destination.ID == "" {
		destination.ID = TelegramDestinationID(destination.ChatID)
	}
	if destination.Label == "" {
		destination.Label = destination.ChatID
	}
	if destination.CreatedAt.IsZero() {
		destination.CreatedAt = now
	}
	destination.UpdatedAt = now
	destination.Enabled = true
	if destination.CreatedByAdminID == 0 {
		destination.CreatedByAdminID = adminID
	}

	routeID := RouteID(provider, sourceID)

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.data.TelegramDestinations[destination.ID]; ok && !existing.CreatedAt.IsZero() {
		destination.CreatedAt = existing.CreatedAt
		if destination.CreatedByAdminID == 0 {
			destination.CreatedByAdminID = existing.CreatedByAdminID
		}
	}
	s.data.TelegramDestinations[destination.ID] = destination

	route, ok := s.data.Routes[routeID]
	if !ok {
		route = Route{
			ID:               routeID,
			Provider:         provider,
			SourceID:         sourceID,
			Enabled:          true,
			CreatedAt:        now,
			CreatedByAdminID: adminID,
		}
	}
	route.Enabled = true
	route.UpdatedAt = now
	if !contains(route.TelegramDestination, destination.ID) {
		route.TelegramDestination = append(route.TelegramDestination, destination.ID)
		sort.Strings(route.TelegramDestination)
	}
	s.data.Routes[routeID] = route

	return route, s.persistLocked()
}

func (s *Store) DisconnectTelegramDestination(provider, sourceID, chatID string) (bool, error) {
	if s == nil {
		return false, ErrNotReady
	}
	if provider == "" {
		provider = ProviderRailway
	}
	routeID := RouteID(provider, strings.TrimSpace(sourceID))
	destinationID := TelegramDestinationID(strings.TrimSpace(chatID))

	s.mu.Lock()
	defer s.mu.Unlock()

	route, ok := s.data.Routes[routeID]
	if !ok {
		return false, nil
	}

	if destinationID == TelegramDestinationID("") {
		delete(s.data.Routes, routeID)
		return true, s.persistLocked()
	}

	next := route.TelegramDestination[:0]
	removed := false
	for _, id := range route.TelegramDestination {
		if id == destinationID {
			removed = true
			continue
		}
		next = append(next, id)
	}

	if !removed {
		return false, nil
	}
	if len(next) == 0 {
		delete(s.data.Routes, routeID)
	} else {
		route.TelegramDestination = next
		route.UpdatedAt = time.Now().UTC()
		s.data.Routes[routeID] = route
	}

	return true, s.persistLocked()
}

func (s *Store) TelegramDestinations(provider, sourceID string) []TelegramDestination {
	if s == nil {
		return nil
	}
	if provider == "" {
		provider = ProviderRailway
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	route, ok := s.data.Routes[RouteID(provider, sourceID)]
	if !ok || !route.Enabled {
		return nil
	}

	destinations := make([]TelegramDestination, 0, len(route.TelegramDestination))
	seen := map[string]bool{}
	for _, id := range route.TelegramDestination {
		if seen[id] {
			continue
		}
		seen[id] = true
		destination, ok := s.data.TelegramDestinations[id]
		if !ok || !destination.Enabled {
			continue
		}
		destinations = append(destinations, destination)
	}

	return destinations
}

func RouteID(provider, sourceID string) string {
	return strings.TrimSpace(provider) + ":" + strings.TrimSpace(sourceID)
}

func TelegramDestinationID(chatID string) string {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return ""
	}
	return ProviderTelegram + ":" + chatID
}

func (s *Store) load() error {
	if s.path == "" {
		return ErrNoDurablePath
	}

	bytes, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.data = defaultData()
			return s.persistLocked()
		}
		return err
	}

	if len(bytes) == 0 {
		s.data = defaultData()
		return s.persistLocked()
	}

	if err := json.Unmarshal(bytes, &s.data); err != nil {
		return fmt.Errorf("load config %s: %w", s.path, err)
	}

	normalizeData(&s.data)
	return nil
}

func (s *Store) persistLocked() error {
	normalizeData(&s.data)
	s.data.UpdatedAt = time.Now().UTC()
	if s.data.CreatedAt.IsZero() {
		s.data.CreatedAt = s.data.UpdatedAt
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(bytes); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, s.path)
}

func defaultData() Data {
	now := time.Now().UTC()
	return Data{
		SchemaVersion:        SchemaVersion,
		KnownProjects:        map[string]KnownProject{},
		Routes:               map[string]Route{},
		TelegramDestinations: map[string]TelegramDestination{},
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func normalizeData(data *Data) {
	if data.SchemaVersion == 0 {
		data.SchemaVersion = SchemaVersion
	}
	if data.KnownProjects == nil {
		data.KnownProjects = map[string]KnownProject{}
	}
	if data.Routes == nil {
		data.Routes = map[string]Route{}
	}
	if data.TelegramDestinations == nil {
		data.TelegramDestinations = map[string]TelegramDestination{}
	}
	if data.CreatedAt.IsZero() {
		data.CreatedAt = time.Now().UTC()
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
