package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/atregu/ipfs-publisher/internal/config"
	"github.com/atregu/ipfs-publisher/internal/lockfile"
	"github.com/atregu/ipfs-publisher/internal/logger"
	"github.com/spf13/pflag"
)

const (
	version = "0.1.0"
)

var (
	configPath  string
	showVersion bool
	showHelp    bool
	initConfig  bool
	checkIPFS   bool
	dryRun      bool
	ipfsMode    string
)

func init() {
	pflag.StringVarP(&configPath, "config", "c", "./config.yaml", "Path to config file")
	pflag.BoolVarP(&showVersion, "version", "v", false, "Show version information")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help message")
	pflag.BoolVar(&initConfig, "init", false, "Initialize configuration and generate keys")
	pflag.BoolVar(&checkIPFS, "check-ipfs", false, "Check IPFS connection and exit")
	pflag.BoolVar(&dryRun, "dry-run", false, "Scan and show what would be processed without uploading")
	pflag.StringVar(&ipfsMode, "ipfs-mode", "", "Override IPFS mode from config (external/embedded)")
}

func main() {
	pflag.Parse()

	// Handle version flag
	if showVersion {
		fmt.Printf("ipfs-publisher version %s\n", version)
		os.Exit(0)
	}

	// Handle help flag
	if showHelp {
		printHelp()
		os.Exit(0)
	}

	// Handle init flag
	if initConfig {
		if err := initializeConfig(); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing configuration: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Configuration initialized successfully")
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		fmt.Fprintf(os.Stderr, "Use --init to create a default configuration\n")
		os.Exit(1)
	}

	// Override IPFS mode if specified
	if ipfsMode != "" {
		mode := config.IPFSMode(ipfsMode)
		if mode != config.IPFSModeExternal && mode != config.IPFSModeEmbedded {
			fmt.Fprintf(os.Stderr, "Invalid IPFS mode: %s (must be 'external' or 'embedded')\n", ipfsMode)
			os.Exit(1)
		}
		cfg.IPFS.Mode = mode
	}

	// Initialize logger
	if err := logger.Init(cfg.Logging.Level, cfg.Logging.File, cfg.Logging.MaxSize, cfg.Logging.MaxBackups, cfg.Logging.Console); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
		os.Exit(1)
	}

	logger.Infof("Starting ipfs-publisher version %s", version)
	logger.Infof("IPFS mode: %s", cfg.IPFS.Mode)

	// Acquire lock file
	baseDir := getBaseDir()
	lock := lockfile.New(baseDir)
	if err := lock.Acquire(); err != nil {
		logger.Fatalf("Failed to acquire lock: %v", err)
	}
	defer func() {
		if err := lock.Release(); err != nil {
			logger.Errorf("Failed to release lock: %v", err)
		}
	}()

	logger.Info("Lock acquired successfully")

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Infof("Received signal: %v", sig)
		logger.Info("Shutting down gracefully...")

		// Release lock
		if err := lock.Release(); err != nil {
			logger.Errorf("Failed to release lock during shutdown: %v", err)
		}

		os.Exit(0)
	}()

	// Handle check-ipfs flag
	if checkIPFS {
		logger.Info("Checking IPFS connection...")
		// TODO: Implement IPFS connection check
		logger.Info("IPFS check not yet implemented")
		os.Exit(0)
	}

	// Handle dry-run flag
	if dryRun {
		logger.Info("Running in dry-run mode...")
		// TODO: Implement dry-run logic
		logger.Info("Dry-run not yet implemented")
		os.Exit(0)
	}

	// Main application logic
	logger.Info("Application started successfully")
	logger.Debugf("Monitoring directories: %v", cfg.Directories)
	logger.Debugf("File extensions: %v", cfg.Extensions)

	// TODO: Implement main application logic

	// Keep application running
	select {}
}

func printHelp() {
	fmt.Println("ipfs-publisher - IPFS Media Collection Publisher")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ipfs-publisher [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	pflag.PrintDefaults()
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Initialize configuration")
	fmt.Println("  ipfs-publisher --init")
	fmt.Println()
	fmt.Println("  # Run with custom config")
	fmt.Println("  ipfs-publisher --config ./config.yaml")
	fmt.Println()
	fmt.Println("  # Check IPFS connection")
	fmt.Println("  ipfs-publisher --check-ipfs")
	fmt.Println()
	fmt.Println("  # Dry run to see what would be processed")
	fmt.Println("  ipfs-publisher --dry-run")
	fmt.Println()
	fmt.Println("  # Override IPFS mode")
	fmt.Println("  ipfs-publisher --ipfs-mode embedded")
}

func initializeConfig() error {
	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists: %s", configPath)
	}

	// Create default config directory
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create default config content
	defaultConfig := `# IPFS Media Collection Publisher Configuration

# IPFS node configuration
ipfs:
  # Mode: "external" (use existing IPFS node) or "embedded" (run IPFS inside app)
  mode: "external"
  
  # External node settings (used when mode: external)
  external:
    api_url: "http://localhost:5001"
    timeout: 300  # seconds
    add_options:
      nocopy: false
      pin: true
      chunker: "size-262144"
      raw_leaves: true
  
  # Embedded node settings (used when mode: embedded)
  embedded:
    repo_path: "~/.ipfs_publisher/ipfs-repo"
    swarm_port: 4002
    api_port: 5002
    gateway_port: 8081
    add_options:
      pin: true
      chunker: "size-262144"
      raw_leaves: true
    bootstrap_peers: []
    gc:
      enabled: true
      interval: 86400  # seconds (24 hours)
      min_free_space: 1073741824  # bytes (1GB)

# PubSub configuration (always uses embedded implementation)
pubsub:
  topic: "mdn/collections/announce"
  announce_interval: 3600  # seconds (1 hour)
  bootstrap_peers: []
  listen_port: 0  # 0 = random port

# Directories to monitor
directories:
  - "/path/to/media1"
  - "/path/to/media2"

# File extensions to process (case-insensitive)
extensions:
  - "mp3"
  - "mp4"
  - "mkv"
  - "avi"
  - "flac"
  - "wav"

# Logging
logging:
  level: "info"  # debug, info, warn, error
  file: "~/.ipfs_publisher/logs/app.log"
  max_size: 100  # MB
  max_backups: 5
  console: true

# Application behavior
behavior:
  scan_interval: 10  # seconds
  batch_size: 10
  progress_bar: true
  state_save_interval: 60  # seconds
`

	// Write config file
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Created default configuration at: %s\n", configPath)
	fmt.Println("Please edit the configuration file to add your media directories")

	return nil
}

func getBaseDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ipfs_publisher"
	}

	baseDir := filepath.Join(home, ".ipfs_publisher")

	// Ensure base directory exists
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create base directory: %v\n", err)
		return ".ipfs_publisher"
	}

	return baseDir
}
