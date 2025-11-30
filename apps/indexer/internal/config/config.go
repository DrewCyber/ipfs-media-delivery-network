package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// IPFSMode represents the mode of IPFS operation
type IPFSMode string

const (
	IPFSModeEmbedded IPFSMode = "embedded"
)

// EmbeddedIPFSConfig contains settings for embedded IPFS node
type EmbeddedIPFSConfig struct {
	RepoPath       string   `mapstructure:"repo_path"`
	SwarmPort      int      `mapstructure:"swarm_port"`
	APIPort        int      `mapstructure:"api_port"`
	GatewayPort    int      `mapstructure:"gateway_port"`
	BootstrapPeers []string `mapstructure:"bootstrap_peers"`
	GC             GCConfig `mapstructure:"gc"`
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
	Embedded EmbeddedIPFSConfig `mapstructure:"embedded"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Type string `mapstructure:"type"`
	Path string `mapstructure:"path"`
}

// PubsubConfig contains Pubsub-related configuration
type PubsubConfig struct {
	Topic string `mapstructure:"topic"`
}

// FetcherConfig contains fetcher settings
type FetcherConfig struct {
	RetryAttempts        int `mapstructure:"retry_attempts"`
	RetryIntervalSeconds int `mapstructure:"retry_interval_seconds"`
	ConcurrentDownloads  int `mapstructure:"concurrent_downloads"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level    string `mapstructure:"level"`
	Format   string `mapstructure:"format"`
	Output   string `mapstructure:"output"`
	FilePath string `mapstructure:"file_path"`
}

// Config represents the complete application configuration
type Config struct {
	IPFS     IPFSConfig     `mapstructure:"ipfs"`
	Database DatabaseConfig `mapstructure:"database"`
	Pubsub   PubsubConfig   `mapstructure:"pubsub"`
	Fetcher  FetcherConfig  `mapstructure:"fetcher"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

// Load reads and parses the configuration file
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set config file path
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		// Default config locations
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate and set defaults
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks if the configuration is valid and sets defaults
func (c *Config) Validate() error {
	// Validate IPFS config
	if c.IPFS.Mode != IPFSModeEmbedded {
		return fmt.Errorf("only embedded IPFS mode is supported")
	}

	if c.IPFS.Embedded.RepoPath == "" {
		return fmt.Errorf("ipfs.embedded.repo_path is required")
	}

	// Expand repo path
	if c.IPFS.Embedded.RepoPath[:2] == "./" {
		abs, err := filepath.Abs(c.IPFS.Embedded.RepoPath)
		if err != nil {
			return fmt.Errorf("failed to resolve repo path: %w", err)
		}
		c.IPFS.Embedded.RepoPath = abs
	}

	// Validate database config
	if c.Database.Type != "sqlite" {
		return fmt.Errorf("only sqlite database type is supported")
	}

	if c.Database.Path == "" {
		return fmt.Errorf("database.path is required")
	}

	// Ensure database directory exists
	dbDir := filepath.Dir(c.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}

	// Validate pubsub config
	if c.Pubsub.Topic == "" {
		return fmt.Errorf("pubsub.topic is required")
	}

	// Validate fetcher config with defaults
	if c.Fetcher.RetryAttempts <= 0 {
		c.Fetcher.RetryAttempts = 10
	}
	if c.Fetcher.RetryIntervalSeconds <= 0 {
		c.Fetcher.RetryIntervalSeconds = 60
	}
	if c.Fetcher.ConcurrentDownloads <= 0 {
		c.Fetcher.ConcurrentDownloads = 5
	}

	// Validate logging config with defaults
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "text"
	}
	if c.Logging.Output == "" {
		c.Logging.Output = "stdout"
	}

	// If output is file, ensure log directory exists
	if c.Logging.Output == "file" && c.Logging.FilePath != "" {
		logDir := filepath.Dir(c.Logging.FilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	return nil
}
