package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/ivan100-ivoop/cloudns-sync/internal/cloudns"
	"github.com/ivan100-ivoop/cloudns-sync/internal/config"
	"github.com/ivan100-ivoop/cloudns-sync/internal/dns/providers"
	"github.com/ivan100-ivoop/cloudns-sync/internal/logger"
)

func main() {
	args := os.Args[1:]
	command := "discover"
	if len(args) > 0 && (args[0] == "discover" || args[0] == "create" || args[0] == "sync") {
		command = args[0]
		args = args[1:]
	} else if len(args) > 0 && strings.HasPrefix(args[0], "-") {
		// Discovery is the default command; --apply upgrades direct flag usage to create.
		for _, arg := range args {
			if arg == "--apply" || arg == "--delete-missing" || arg == "--watch" {
				command = "create"
				break
			}
		}
	} else {
		usage()
		os.Exit(2)
	}
	var err error
	if command == "create" || command == "sync" {
		err = create(args)
	} else {
		err = discover(args)
	}
	if err != nil {
		sanitized := logger.Sanitize(err.Error())
		if errorLogger, logErr := logger.New("./logs", "error-{date}.log"); logErr == nil {
			errorLogger.Error("discover", "DNS", "", "", fmt.Errorf("%s", sanitized))
			_ = errorLogger.Close()
		}
		fmt.Fprintln(os.Stderr, sanitized)
		os.Exit(1)
	}
}

func discover(args []string) error {
	cfg, zones, sourcePath, provider, err := loadZones(args)
	if err != nil {
		return err
	}
	fmt.Printf("DNS provider: %s\n", provider)
	fmt.Printf("Configuration: %s\n\n", sourcePath)
	fmt.Printf("Discovered master zones: %d\n\n", len(zones))
	for _, zone := range zones {
		fmt.Println(zone)
	}
	fmt.Printf("\nDiscovery completed successfully: %d master zones found.\n", len(zones))
	_ = cfg
	return nil
}

func create(args []string) error {
	flags := flag.NewFlagSet("create", flag.ContinueOnError)
	apply := flags.Bool("apply", false, "actually create slave zones; without this flag only show the plan")
	configPath := flags.String("config", "", "path to YAML configuration")
	configPathShort := flags.String("c", "", "alias for --config")
	provider := flags.String("provider", "", "DNS provider name")
	backend := flags.String("backend", "", "deprecated alias for --provider")
	namedPath := flags.String("named", "", "override named provider config_file")
	powerDNSCommand := flags.String("powerdns-command", "", "PowerDNS administration command")
	watch := flags.Bool("watch", false, "keep synchronizing at settings.sync_interval")
	deleteMissing := flags.Bool("delete-missing", false, "delete missing remote slave zones")

	if err := flags.Parse(args); err != nil {
		return err
	}
	if *configPath == "" {
		*configPath = *configPathShort
	}
	loadArgs := []string{}
	if *configPath != "" {
		loadArgs = append(loadArgs, "--config", *configPath)
	}
	if *provider != "" {
		loadArgs = append(loadArgs, "--provider", *provider)
	} else if *backend != "" {
		loadArgs = append(loadArgs, "--backend", *backend)
	}
	if *namedPath != "" {
		loadArgs = append(loadArgs, "--named", *namedPath)
	}
	if *powerDNSCommand != "" {
		loadArgs = append(loadArgs, "--powerdns-command", *powerDNSCommand)
	}
	runOnce := func() error {
		cfg, zones, _, _, err := loadZones(loadArgs)
		if err != nil {
			return err
		}
		if len(cfg.Settings.MasterIP) == 0 {
			return fmt.Errorf("settings.master_ip must contain at least one master IP")
		}
		if *apply && cfg.Settings.DryRun {
			return fmt.Errorf("settings.dry_run is true; set it to false before using --apply")
		}
		client := cloudns.Client{APIURL: cfg.CloudNS.APIURL, AuthID: cfg.CloudNS.AuthID, AuthPassword: cfg.CloudNS.AuthPassword}
		excluded := make([]string, 0, len(cfg.Settings.ExcludeZones))
		for _, zone := range cfg.Settings.ExcludeZones {
			pattern := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(zone)), ".")
			if _, err := path.Match(pattern, "example.com"); err != nil {
				return fmt.Errorf("invalid exclude_zones pattern %q: %w", zone, err)
			}
			excluded = append(excluded, pattern)
		}
		deleteEnabled := *deleteMissing || cfg.Settings.DeleteMissing
		existing := map[string]cloudns.Zone{}
		if *apply || deleteEnabled {
			existing, err = client.ListZones(context.Background())
			if err != nil {
				return err
			}
			filterExcludedZones(existing, excluded)
		}
		localZones := make(map[string]struct{}, len(zones))
		for _, zone := range zones {
			localZones[strings.TrimSuffix(strings.ToLower(zone), ".")] = struct{}{}
		}
		created, skipped, failed := 0, 0, 0
		for _, zone := range zones {
			key := strings.TrimSuffix(strings.ToLower(zone), ".")
			if matchesExcludedZone(key, excluded) {
				fmt.Printf("Skipping excluded zone: %s\n", zone)
				skipped++
				continue
			}
			if _, exists := existing[key]; exists {
				fmt.Printf("Already exists in ClouDNS: %s\n", zone)
				skipped++
				continue
			}
			masterIP := cfg.Settings.MasterIP[0]
			if !*apply {
				fmt.Printf("DRY RUN: would create slave zone %s with master %s\n", zone, masterIP)
				continue
			}
			if err := client.RegisterSlaveZone(context.Background(), zone, masterIP); err != nil {
				failed++
				fmt.Printf("Failed to create slave zone %s: %s\n", zone, logger.Sanitize(err.Error()))
				continue
			}
			fmt.Printf("Created slave zone: %s\n", zone)
			created++
		}
		deleted := 0
		if deleteEnabled {
			if !*apply {
				for key, remote := range existing {
					if _, present := localZones[key]; !present && remote.Type == "slave" && !matchesExcludedZone(key, excluded) {
						fmt.Printf("DRY RUN: would delete missing slave zone %s\n", remote.Name)
					}
				}
			} else {
				for key, remote := range existing {
					if _, present := localZones[key]; present || remote.Type != "slave" || matchesExcludedZone(key, excluded) {
						continue
					}
					if err := client.DeleteZone(context.Background(), remote.Name); err != nil {
						failed++
						fmt.Printf("Failed to delete missing slave zone %s: %s\n", remote.Name, logger.Sanitize(err.Error()))
						continue
					}
					fmt.Printf("Deleted missing slave zone: %s\n", remote.Name)
					deleted++
				}
			}
		}
		if !*apply {
			fmt.Printf("No changes made. Use --apply to create or delete zones.\n")
			return nil
		}
		fmt.Printf("Synchronization completed: %d created, %d deleted, %d skipped, %d failed.\n", created, deleted, skipped, failed)
		if failed > 0 {
			return fmt.Errorf("synchronization completed with %d failed zones", failed)
		}
		return nil
	}
	if !*watch {
		return runOnce()
	}
	if *configPath == "" {
		return fmt.Errorf("--watch requires --config")
	}
	watchConfig, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if watchConfig.Settings.RunOnStart {
		if err := runOnce(); err != nil {
			fmt.Fprintln(os.Stderr, logger.Sanitize(err.Error()))
		}
	}
	ticker := time.NewTicker(watchConfig.Settings.SyncInterval.Duration())
	defer ticker.Stop()
	for range ticker.C {
		if err := runOnce(); err != nil {
			fmt.Fprintln(os.Stderr, logger.Sanitize(err.Error()))
		}
	}
	return nil
}

func matchesExcludedZone(zone string, patterns []string) bool {
	for _, pattern := range patterns {
		matched, err := path.Match(pattern, zone)
		if err == nil && matched {
			return true
		}
	}
	return false
}

func filterExcludedZones(zones map[string]cloudns.Zone, patterns []string) {
	for key := range zones {
		if matchesExcludedZone(key, patterns) {
			delete(zones, key)
		}
	}
}

func loadZones(args []string) (config.Config, []string, string, string, error) {
	flags := flag.NewFlagSet("discover", flag.ContinueOnError)
	configPath := flags.String("config", "", "path to YAML configuration")
	configPathShort := flags.String("c", "", "alias for --config")
	provider := flags.String("provider", "", "DNS provider name, for example named or powerdns")
	backend := flags.String("backend", "", "deprecated alias for --provider")
	namedPath := flags.String("named", "", "override named provider config_file")
	powerDNSCommand := flags.String("powerdns-command", "", "PowerDNS administration command")
	var cfg config.Config
	if err := flags.Parse(args); err != nil {
		return config.Config{}, nil, "", "", err
	}
	if *provider == "" {
		*provider = *backend
	}
	if *configPath == "" {
		*configPath = *configPathShort
	}
	configDir := ""
	if *configPath != "" {
		configDir = filepath.Dir(*configPath)
		loaded, err := config.Load(*configPath)
		if err != nil {
			return config.Config{}, nil, "", "", err
		}
		cfg = loaded
		if *provider == "" {
			*provider = cfg.Settings.DNSProvider
		}
	}
	if *provider == "" {
		*provider = "named"
	}
	providerDirectory := filepath.Join(configDir, "providers")
	if _, statErr := os.Stat(providerDirectory); os.IsNotExist(statErr) {
		providerDirectory = filepath.Join(configDir, "configs", "providers")
	}
	providerDefinitions, err := config.LoadProviders(providerDirectory)
	if err != nil {
		return config.Config{}, nil, "", "", err
	}
	providerSettings, ok := providerDefinitions[strings.ToLower(strings.TrimSpace(*provider))]
	if !ok {
		return config.Config{}, nil, "", "", fmt.Errorf("provider %q was not found in %s", *provider, providerDirectory)
	}
	providerSettings.ConfigFile = resolveRelative(configDir, providerSettings.ConfigFile)
	source, sourcePath, err := providers.New(providers.Options{
		Name:       *provider,
		ConfigFile: *namedPath,
		Command:    *powerDNSCommand,
		Settings:   providerSettings,
	})
	if err != nil {
		return config.Config{}, nil, "", "", err
	}
	zones, err := source.ListZones(context.Background())
	if err != nil {
		return config.Config{}, nil, "", "", err
	}
	return cfg, zones, sourcePath, *provider, nil
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: cloudns-sync [discover|create] [-c path|--config path] [--provider named|powerdns] [--apply] [--watch] [--delete-missing]")
}

func resolveRelative(base, path string) string {
	if path == "" || filepath.IsAbs(path) || base == "" {
		return path
	}
	return filepath.Join(base, path)
}
