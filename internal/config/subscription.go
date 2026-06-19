package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Subscription represents a single remote subscription.
type Subscription struct {
	Name       string            `toml:"name"`
	URL        string            `toml:"url"`
	UserAgent  string            `toml:"user_agent"`
	AutoUpdate bool              `toml:"auto_update"`
	Headers    map[string]string `toml:"headers"`
	Proxy      string            `toml:"proxy"`

	// Runtime state (managed by tool)
	LastUpdate string `toml:"last_update"`
	LastStatus string `toml:"last_status"`
	LastError  string `toml:"last_error"`
	LastSize   int64  `toml:"last_size"`
	NextUpdate string `toml:"next_update"`
	CreatedAt  string `toml:"created_at"`
}

// SubscriptionsFile holds the subscriptions list.
type SubscriptionsFile struct {
	ActiveSubscription string         `toml:"active_subscription"`
	Subscriptions      []Subscription `toml:"subscriptions"`
	path               string
}

// SubscriptionsPath returns the path to subscriptions.toml under mihoroDir.
func SubscriptionsPath(mihoroDir string) string {
	return filepath.Join(mihoroDir, "subscriptions.toml")
}

// SubDownloadDir returns the directory for downloaded subscription files.
func SubDownloadDir(mihoroDir string) string {
	return filepath.Join(mihoroDir, "subscriptions")
}

// SubDownloadPath returns the path for a subscription's downloaded file.
func SubDownloadPath(mihoroDir, name string) string {
	return filepath.Join(SubDownloadDir(mihoroDir), name+".yaml")
}

// LoadSubscriptions reads subscriptions.toml, returning defaults if absent.
func LoadSubscriptions(dir string) (*SubscriptionsFile, error) {
	path := SubscriptionsPath(dir)
	sf := &SubscriptionsFile{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sf, nil
		}
		return nil, fmt.Errorf("read subscriptions: %w", err)
	}

	if err := toml.Unmarshal(data, sf); err != nil {
		return nil, fmt.Errorf("parse subscriptions: %w", err)
	}
	sf.path = path
	return sf, nil
}

// Save writes subscriptions.toml to disk (atomic).
func (sf *SubscriptionsFile) Save() error {
	if sf.path == "" {
		return fmt.Errorf("subscriptions path not set")
	}

	if err := os.MkdirAll(filepath.Dir(sf.path), 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	f, err := os.CreateTemp(filepath.Dir(sf.path), ".subscriptions-*.toml")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := f.Name()
	defer os.Remove(tmpName)

	if err := toml.NewEncoder(f).Encode(sf); err != nil {
		f.Close()
		return fmt.Errorf("encode subscriptions: %w", err)
	}
	f.Close()

	if err := os.Rename(tmpName, sf.path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// Find returns a subscription by name, or nil if not found.
func (sf *SubscriptionsFile) Find(name string) *Subscription {
	for i := range sf.Subscriptions {
		if sf.Subscriptions[i].Name == name {
			return &sf.Subscriptions[i]
		}
	}
	return nil
}

// Add appends a new subscription.
func (sf *SubscriptionsFile) Add(s Subscription) error {
	if s.Name == "" {
		return fmt.Errorf("subscription name is required")
	}
	if s.URL == "" {
		return fmt.Errorf("subscription URL is required")
	}
	if sf.Find(s.Name) != nil {
		return fmt.Errorf("subscription %q already exists", s.Name)
	}

	s.CreatedAt = time.Now().Format(time.RFC3339)
	if s.UserAgent == "" {
		s.UserAgent = "clash/mihoro-go"
	}
	sf.Subscriptions = append(sf.Subscriptions, s)
	return nil
}

// Remove deletes a subscription by name.
func (sf *SubscriptionsFile) Remove(name string) error {
	for i := range sf.Subscriptions {
		if sf.Subscriptions[i].Name == name {
			sf.Subscriptions = append(sf.Subscriptions[:i], sf.Subscriptions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("subscription %q not found", name)
}

// Update replaces a subscription by name, preserving runtime state.
func (sf *SubscriptionsFile) Update(name string, s Subscription) error {
	for i := range sf.Subscriptions {
		if sf.Subscriptions[i].Name == name {
			s.LastUpdate = sf.Subscriptions[i].LastUpdate
			s.LastStatus = sf.Subscriptions[i].LastStatus
			s.LastError = sf.Subscriptions[i].LastError
			s.LastSize = sf.Subscriptions[i].LastSize
			s.NextUpdate = sf.Subscriptions[i].NextUpdate
			s.CreatedAt = sf.Subscriptions[i].CreatedAt
			sf.Subscriptions[i] = s
			return nil
		}
	}
	return fmt.Errorf("subscription %q not found", name)
}

// Active returns the active subscription, or nil if none.
func (sf *SubscriptionsFile) Active() *Subscription {
	return sf.Find(sf.ActiveSubscription)
}
