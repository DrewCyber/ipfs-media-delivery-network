package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

// IPFSMode represents the mode of IPFS operation
type IPFSMode string

const (
	IPFSModeExternal IPFSMode = "external"
	IPFSModeEmbedded IPFSMode = "embedded"
)

// ExternalIPFSConfig contains settings for external IPFS node
type ExternalIPFSConfig struct {
	APIURL  string                 `mapstructure:"api_url"`
	Timeout int                    `mapstructure:"timeout"`
	Options map[string]interface{} `mapstructure:"add_options"`
}

// EmbeddedIPFSConfig contains settings for embedded IPFS node
type EmbeddedIPFSConfig struct {
	RepoPath       string                 `mapstructure:"repo_path"`
	SwarmPort      int                    `mapstructure:"swarm_port"`
	APIPort        int                    `mapstructure:"api_port"`
	GatewayPort    int                    `mapstructure:"gateway_port"`
	Options        map[string]interface{} `mapstructure:"add_options"`
	BootstrapPeers []string               `mapstructure:"bootstrap_peers"`
	GC             GCConfig               `mapstructure:"gc"`
}

// GCConfig contains garbage collection settings
type GCConfig struct {
	Enabled      bool  `mapstructure:"enabled"`
	Interval     int64 `mapstructure:"interval"`
	MinFreeSpace int64 `mapstructure:"min_free_space"`
}

// IPFSConfig contains IPFS-related configuration
type IPFSConfig struct {
	Mode     IPFSMode           `mapstructure:"mode"`
	External ExternalIPFSConfig `mapstructure:"external"`
	Embedded EmbeddedIPFSConfig `mapstructure:"embedded"`
}

// PubsubConfig contains Pubsub-related configuration
type PubsubConfig struct {
	Topic            string   `mapstructure:"topic"`
	AnnounceInterval int      `mapstructure:"announce_interval"`
	BootstrapPeers   []string `mapstructure:"bootstrap_peers"`
	ListenPort       int      `mapstructure:"listen_port"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	File       string `mapstructure:"file"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	Console    bool   `mapstructure:"console"`
}

// BehaviorConfig contains application behavior settings
type BehaviorConfig struct {
	ScanInterval      int  `mapstructure:"scan_interval"`
	BatchSize         int  `mapstructure:"batch_size"`
	ProgressBar       bool `mapstructure:"progress_bar"`
	StateSaveInterval int  `mapstructure:"state_save_interval"`
}

// Config represents the complete application configuration
type Config struct {
	IPFS        IPFSConfig     `mapstructure:"ipfs"`
	Pubsub      PubsubConfig   `mapstructure:"pubsub"`
	Directories []string       `mapstructure:"directories"`
	Extensions  []string       `mapstructure:"extensions"`
	Logging     LoggingConfig  `mapstructure:"logging"`
	Behavior    BehaviorConfig `mapstructure:"behavior"`
}

// Load loads configuration from the specified file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Expand tilde in config path
	if strings.HasPrefix(configPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(home, configPath[1:])
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s", configPath)
	}

	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Expand tilde in paths
	cfg.expandPaths()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	v.SetDefault("ipfs.mode", "external")
	v.SetDefault("ipfs.external.api_url", "http://localhost:5001")
	v.SetDefault("ipfs.external.timeout", 300)
	v.SetDefault("ipfs.embedded.swarm_port", 4002)
	v.SetDefault("ipfs.embedded.api_port", 5002)
	v.SetDefault("ipfs.embedded.gateway_port", 8081)
	v.SetDefault("ipfs.embedded.repo_path", "~/.ipfs_publisher/ipfs-repo")
	v.SetDefault("pubsub.topic", "mdn/collections/announce")
	v.SetDefault("pubsub.announce_interval", 3600)
	v.SetDefault("pubsub.listen_port", 0)
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.file", "~/.ipfs_publisher/logs/app.log")
	v.SetDefault("logging.max_size", 100)
	v.SetDefault("logging.max_backups", 5)
	v.SetDefault("logging.console", true)
	v.SetDefault("behavior.scan_interval", 10)
	v.SetDefault("behavior.batch_size", 10)
	v.SetDefault("behavior.progress_bar", true)
	v.SetDefault("behavior.state_save_interval", 60)
}

// expandPaths expands ~ in file paths
func (c *Config) expandPaths() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	// Expand logging file path
	if strings.HasPrefix(c.Logging.File, "~") {
		c.Logging.File = filepath.Join(home, c.Logging.File[1:])
	}

	// Expand embedded repo path
	if strings.HasPrefix(c.IPFS.Embedded.RepoPath, "~") {
		c.IPFS.Embedded.RepoPath = filepath.Join(home, c.IPFS.Embedded.RepoPath[1:])
	}

	// Expand directories
	for i, dir := range c.Directories {
		if strings.HasPrefix(dir, "~") {
			c.Directories[i] = filepath.Join(home, dir[1:])
		}
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate IPFS mode
	if c.IPFS.Mode != IPFSModeExternal && c.IPFS.Mode != IPFSModeEmbedded {
		return fmt.Errorf("invalid IPFS mode: %s (must be 'external' or 'embedded')", c.IPFS.Mode)
	}

	// Validate ports for embedded mode
	if c.IPFS.Mode == IPFSModeEmbedded {
		if err := validatePort(c.IPFS.Embedded.SwarmPort, "swarm_port"); err != nil {
			return err
		}
		if err := validatePort(c.IPFS.Embedded.APIPort, "api_port"); err != nil {
			return err
		}
		if err := validatePort(c.IPFS.Embedded.GatewayPort, "gateway_port"); err != nil {
			return err
		}

		// Check for duplicate ports
		ports := map[int]string{
			c.IPFS.Embedded.SwarmPort:   "swarm_port",
			c.IPFS.Embedded.APIPort:     "api_port",
			c.IPFS.Embedded.GatewayPort: "gateway_port",
		}
		if len(ports) < 3 {
			return fmt.Errorf("embedded IPFS ports must be unique")
		}
	}

	// Validate directories
	if len(c.Directories) == 0 {
		return fmt.Errorf("at least one directory must be configured")
	}

	for _, dir := range c.Directories {
		if dir == "" {
			return fmt.Errorf("directory path cannot be empty")
		}
		// Check if directory exists
		info, err := os.Stat(dir)
		if err != nil {
			return fmt.Errorf("directory %s: %w", dir, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("%s is not a directory", dir)
		}
	}

	// Validate extensions
	if len(c.Extensions) == 0 {
		return fmt.Errorf("at least one file extension must be configured")
	}

	// Validate logging level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid logging level: %s", c.Logging.Level)
	}

	// Validate behavior values
	if c.Behavior.ScanInterval <= 0 {
		return fmt.Errorf("scan_interval must be positive")
	}
	if c.Behavior.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be positive")
	}
	if c.Behavior.StateSaveInterval <= 0 {
		return fmt.Errorf("state_save_interval must be positive")
	}

	return nil
}

// validatePort checks if a port number is valid
func validatePort(port int, name string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535, got %d", name, port)
	}
	return nil
}
