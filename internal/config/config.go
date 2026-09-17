package config

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	CloudNS  CloudNSConfig  `yaml:"cloudns"`
	Settings SettingsConfig `yaml:"settings"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type CloudNSConfig struct {
	AuthID       string `yaml:"auth_id"`
	AuthPassword string `yaml:"auth_password"`
	APIURL       string `yaml:"api_url"`
}

type SettingsConfig struct {
	MasterIP      []string `yaml:"master_ip"`
	RunOnStart    bool     `yaml:"run_on_start"`
	DryRun        bool     `yaml:"dry_run"`
	SyncInterval  Duration `yaml:"sync_interval"`
	DNSProvider   string   `yaml:"dns_provider"`
	ExcludeZones  []string `yaml:"exclude_zones"`
	DeleteMissing bool     `yaml:"delete_missing"`
}

// ProviderConfig contains only provider-specific discovery settings. The
// selected provider decides which fields are meaningful.
type ProviderConfig struct {
	Provider   string   `yaml:"provider"`
	Type       string   `yaml:"type"`
	ConfigFile string   `yaml:"config_file"`
	Command    string   `yaml:"command"`
	Args       []string `yaml:"args"`
}

type CPanelConfig struct {
	Enabled  bool `yaml:"enabled"`
	Named    bool `yaml:"named"`
	PowerDNS bool `yaml:"powerdns"`
}

type BindConfig struct {
	NamedConf string `yaml:"named_conf"`
}

type PowerDNSConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args"`
}

type LoggingConfig struct {
	Directory string `yaml:"directory"`
	Level     string `yaml:"level"`
	ErrorFile string `yaml:"error_file"`
}

// Duration keeps YAML duration values as time.Duration after strict parsing.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	parsed, err := time.ParseDuration(strings.TrimSpace(value.Value))
	if err != nil {
		return fmt.Errorf("sync_interval: %w", err)
	}
	if parsed < time.Second {
		return fmt.Errorf("sync_interval must be at least 1s")
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) Duration() time.Duration { return time.Duration(d) }

func Load(path string) (Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}
	return Parse(contents)
}

func LoadProvider(path string) (ProviderConfig, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ProviderConfig{}, fmt.Errorf("read provider config %q: %w", path, err)
	}
	var provider ProviderConfig
	if err := yaml.Unmarshal(contents, &provider); err != nil {
		return ProviderConfig{}, fmt.Errorf("parse provider config %q: %w", path, err)
	}
	return provider, provider.Validate()
}

// LoadProviders reads every YAML provider definition in a directory. The
// provider field inside each file is the canonical provider name.
func LoadProviders(directory string) (map[string]ProviderConfig, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read provider directory %q: %w", directory, err)
	}
	loaded := make(map[string]ProviderConfig)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension != ".yml" && extension != ".yaml" {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		provider, err := LoadProvider(path)
		if err != nil {
			return nil, err
		}
		name := strings.ToLower(strings.TrimSpace(provider.Provider))
		if name == "" {
			return nil, fmt.Errorf("provider config %q must define provider", path)
		}
		if _, exists := loaded[name]; exists {
			return nil, fmt.Errorf("duplicate provider %q in %q", name, directory)
		}
		loaded[name] = provider
	}
	return loaded, nil
}

func Parse(contents []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(contents, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	provider := strings.ToLower(strings.TrimSpace(c.Settings.DNSProvider))
	if provider != "" && provider != "named" && provider != "bind" && provider != "powerdns" {
		return fmt.Errorf("settings.dns_provider must be named, bind, or powerdns")
	}
	if len(c.Settings.MasterIP) == 0 {
		return errors.New("settings.master_ip must contain at least one IP address")
	}
	for index, address := range c.Settings.MasterIP {
		if net.ParseIP(strings.TrimSpace(address)) == nil {
			return fmt.Errorf("settings.master_ip[%d] is not a valid IP address", index)
		}
	}
	if time.Duration(c.Settings.SyncInterval) < time.Second {
		return errors.New("settings.sync_interval must be at least 1s")
	}
	if c.Logging.Directory == "" || c.Logging.ErrorFile == "" {
		return errors.New("logging.directory and logging.error_file are required")
	}
	if c.Logging.Level == "" {
		return errors.New("logging.level is required")
	}
	return nil
}

func (c ProviderConfig) Validate() error {
	provider := strings.ToLower(strings.TrimSpace(c.Provider))
	if provider != "" && provider != "named" && provider != "bind" && provider != "powerdns" {
		return fmt.Errorf("provider must be named, bind, or powerdns")
	}
	if strings.TrimSpace(c.Type) == "" {
		return errors.New("provider type is required")
	}
	return nil
}
