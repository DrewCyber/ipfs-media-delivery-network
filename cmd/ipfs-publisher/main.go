package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/atregu/ipfs-publisher/internal/config"
	"github.com/atregu/ipfs-publisher/internal/index"
	"github.com/atregu/ipfs-publisher/internal/ipfs"
	"github.com/atregu/ipfs-publisher/internal/keys"
	"github.com/atregu/ipfs-publisher/internal/lockfile"
	"github.com/atregu/ipfs-publisher/internal/logger"
	"github.com/atregu/ipfs-publisher/internal/pubsub"
	"github.com/atregu/ipfs-publisher/internal/scanner"
	"github.com/atregu/ipfs-publisher/internal/state"
	progressbar "github.com/schollz/progressbar/v3"
	"github.com/spf13/pflag"
)

const (
	version = "0.1.0"
)

var (
	configPath   string
	showVersion  bool
	showHelp     bool
	initConfig   bool
	checkIPFS    bool
	dryRun       bool
	ipfsMode     string
	testUpload   string
	testIPNS     bool
	testPubSub   bool
	showPeerInfo bool
)

func init() {
	pflag.StringVarP(&configPath, "config", "c", "./config.yaml", "Path to config file")
	pflag.BoolVarP(&showVersion, "version", "v", false, "Show version information")
	pflag.BoolVarP(&showHelp, "help", "h", false, "Show help message")
	pflag.BoolVar(&initConfig, "init", false, "Initialize configuration and generate keys")
	pflag.BoolVar(&checkIPFS, "check-ipfs", false, "Check IPFS connection and exit")
	pflag.BoolVar(&dryRun, "dry-run", false, "Scan and show what would be processed without uploading")
	pflag.StringVar(&ipfsMode, "ipfs-mode", "", "Override IPFS mode from config (external/embedded)")
	pflag.StringVar(&testUpload, "test-upload", "", "Upload a test file to IPFS and exit")
	pflag.BoolVar(&testIPNS, "test-ipns", false, "Test IPNS publish and resolve")
	pflag.BoolVar(&testPubSub, "test-pubsub", false, "Test PubSub announcements")
	pflag.BoolVar(&showPeerInfo, "peer-info", false, "Show IPFS peer addresses and exit")
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

	// Handle peer-info flag (must be after config loaded)
	if showPeerInfo {
		if err := showNodePeerInfo(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error showing peer info: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
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

	// Create IPFS client
	ipfsClient, err := createIPFSClient(cfg)
	if err != nil {
		logger.Fatalf("Failed to create IPFS client: %v", err)
	}
	defer ipfsClient.Close()

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
		if err := checkIPFSConnection(ipfsClient); err != nil {
			logger.Fatalf("IPFS check failed: %v", err)
		}
		os.Exit(0)
	}

	// Handle test-upload flag
	if testUpload != "" {
		if err := testFileUpload(ipfsClient, testUpload, cfg); err != nil {
			logger.Fatalf("Test upload failed: %v", err)
		}
		os.Exit(0)
	}

	// Handle test-ipns flag
	if testIPNS {
		if err := testIPNSOperations(ipfsClient); err != nil {
			logger.Fatalf("Test IPNS failed: %v", err)
		}
		os.Exit(0)
	}

	// Handle test-pubsub flag
	if testPubSub {
		if err := testPubSubOperations(cfg); err != nil {
			logger.Fatalf("Test PubSub failed: %v", err)
		}
		os.Exit(0)
	}

	// Handle dry-run flag
	if dryRun {
		if err := runScan(cfg, nil, true); err != nil {
			logger.Fatalf("Scan failed: %v", err)
		}
		os.Exit(0)
	}

	// Main application logic
	logger.Info("Starting main application...")
	if err := runScan(cfg, ipfsClient, false); err != nil {
		logger.Fatalf("Failed to process files: %v", err)
	}

	logger.Info("Processing complete!")
	logger.Info("Application started successfully")
	logger.Debugf("Monitoring directories: %v", cfg.Directories)
	logger.Debugf("File extensions: %v", cfg.Extensions)

	// Start periodic PubSub announcements if enabled
	if cfg.Pubsub.Enabled {
		logger.Info("PubSub announcements enabled - starting periodic publisher")
		logger.Infof("Announcement interval: %v seconds", cfg.Pubsub.AnnounceInterval)

		if cfg.IPFS.Mode == config.IPFSModeEmbedded {
			// Embedded mode: Use embedded IPFS node's PubSub
			logger.Info("Using embedded IPFS node's PubSub (same libp2p instance)")

			// Publish initial announcement after a short delay
			go func() {
				time.Sleep(5 * time.Second) // Give node time to connect to peers
				stateManager := state.New(filepath.Join(getBaseDir(), "state.json"))
				if err := stateManager.Load(); err == nil {
					ipns := stateManager.GetIPNS()
					if ipns != "" {
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						if err := publishAnnouncementViaIPFS(ctx, ipfsClient, cfg.Pubsub.Topic, ipns, len(stateManager.GetAllFiles()), stateManager.GetVersion()); err != nil {
							logger.Warnf("Failed to publish initial announcement: %v", err)
						} else {
							logger.Info("✓ Initial announcement published")
						}
						cancel()
					}
				}
			}()

			go runPeriodicAnnouncementsEmbedded(cfg, ipfsClient)
		} else {
			// External mode: Create standalone libp2p PubSub node
			logger.Info("Using standalone libp2p PubSub node (IPFS Desktop doesn't support PubSub API)")

			pubsubNode, err := initPubSubNode(cfg)
			if err != nil {
				logger.Fatalf("Failed to initialize standalone PubSub node: %v", err)
			}
			defer pubsubNode.Stop()

			// Publish initial announcement after a short delay
			go func() {
				time.Sleep(5 * time.Second) // Give node time to connect to peers
				stateManager := state.New(filepath.Join(getBaseDir(), "state.json"))
				if err := stateManager.Load(); err == nil {
					ipns := stateManager.GetIPNS()
					if ipns != "" {
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						if err := publishAnnouncementViaStandalone(ctx, pubsubNode, cfg.Pubsub.Topic, ipns, len(stateManager.GetAllFiles()), stateManager.GetVersion()); err != nil {
							logger.Warnf("Failed to publish initial announcement: %v", err)
						} else {
							logger.Info("✓ Initial announcement published")
						}
						cancel()
					}
				}
			}()

			go runPeriodicAnnouncementsStandalone(cfg, pubsubNode)
		}
	}

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
  enabled: true  # Enable PubSub announcements
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

// createIPFSClient creates an IPFS client based on configuration
func createIPFSClient(cfg *config.Config) (ipfs.Client, error) {
	if cfg.IPFS.Mode == config.IPFSModeExternal {
		logger.Infof("Connecting to external IPFS node at %s", cfg.IPFS.External.APIURL)
		timeout := time.Duration(cfg.IPFS.External.Timeout) * time.Second
		client, err := ipfs.NewExternalClient(cfg.IPFS.External.APIURL, timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to create external IPFS client: %w", err)
		}
		return client, nil
	}

	// Embedded mode
	if cfg.IPFS.Mode == config.IPFSModeEmbedded {
		logger.Info("Starting embedded IPFS node...")
		client, err := ipfs.NewEmbeddedClient(&cfg.IPFS.Embedded)
		if err != nil {
			return nil, fmt.Errorf("failed to create embedded IPFS client: %w", err)
		}

		// Start the embedded node
		if err := client.Start(); err != nil {
			return nil, fmt.Errorf("failed to start embedded IPFS node: %w", err)
		}

		return client, nil
	}

	return nil, fmt.Errorf("invalid IPFS mode: %s", cfg.IPFS.Mode)
}

// initPubSub initializes PubSub node and publisher
func initPubSub(cfg *config.Config) (*pubsub.Publisher, error) {
	log := logger.Get()

	// Create PubSub node config
	nodeConfig := &pubsub.Config{
		Topic:          cfg.Pubsub.Topic,
		ListenPort:     cfg.Pubsub.ListenPort,
		BootstrapPeers: cfg.Pubsub.BootstrapPeers,
	}

	node, err := pubsub.NewNode(nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create PubSub node: %w", err)
	}

	// Start the node
	if err := node.Start(nodeConfig); err != nil {
		return nil, fmt.Errorf("failed to start PubSub node: %w", err)
	}

	// Load or generate keys for message signing
	keyMgr := keys.New(filepath.Join(getBaseDir(), "keys"))
	if err := keyMgr.Initialize(); err != nil {
		node.Stop()
		return nil, fmt.Errorf("failed to initialize keys: %w", err)
	}

	privateKey := keyMgr.GetPrivateKey()

	// Create publisher
	announceInterval := time.Duration(cfg.Pubsub.AnnounceInterval) * time.Second
	publisherConfig := &pubsub.PublisherConfig{
		AnnounceInterval: announceInterval,
	}

	publisher := pubsub.NewPublisher(node, privateKey, publisherConfig)

	// Start periodic announcements
	if err := publisher.Start(); err != nil {
		node.Stop()
		return nil, fmt.Errorf("failed to start publisher: %w", err)
	}

	log.Infof("PubSub node started on port %d", cfg.Pubsub.ListenPort)
	log.Infof("Topic: %s", cfg.Pubsub.Topic)
	log.Infof("Periodic announcements every %v", announceInterval)

	return publisher, nil
}

// checkIPFSConnection checks if IPFS node is available
func checkIPFSConnection(client ipfs.Client) error {
	logger.Info("Checking IPFS connection...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.IsAvailable(ctx); err != nil {
		logger.Errorf("IPFS node not available: %v", err)
		return err
	}

	// Get additional info
	if extClient, ok := client.(*ipfs.ExternalClient); ok {
		version, err := extClient.GetVersion()
		if err != nil {
			logger.Warnf("Failed to get IPFS version: %v", err)
		} else {
			logger.Infof("✓ Connected to IPFS node")
			logger.Infof("  Version: %s", version)
		}

		id, err := extClient.GetID()
		if err != nil {
			logger.Warnf("Failed to get IPFS node ID: %v", err)
		} else {
			logger.Infof("  Node ID: %s", id)
		}
	} else {
		logger.Info("✓ Connected to IPFS node")
	}

	return nil
}

// testFileUpload uploads a test file to IPFS
func testFileUpload(client ipfs.Client, filePath string, cfg *config.Config) error {
	logger.Infof("Testing file upload: %s", filePath)

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Prepare add options from config
	addOpts := ipfs.AddOptions{
		Pin:       true,
		NoCopy:    false,
		Chunker:   "size-262144",
		RawLeaves: true,
	}

	// Get options from config if available
	if cfg.IPFS.Mode == config.IPFSModeExternal {
		if val, ok := cfg.IPFS.External.Options["pin"].(bool); ok {
			addOpts.Pin = val
		}
		if val, ok := cfg.IPFS.External.Options["nocopy"].(bool); ok {
			addOpts.NoCopy = val
		}
		if val, ok := cfg.IPFS.External.Options["chunker"].(string); ok {
			addOpts.Chunker = val
		}
		if val, ok := cfg.IPFS.External.Options["raw_leaves"].(bool); ok {
			addOpts.RawLeaves = val
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	logger.Info("Uploading to IPFS...")
	result, err := client.Add(ctx, file, filepath.Base(filePath), addOpts)
	if err != nil {
		return fmt.Errorf("failed to add file to IPFS: %w", err)
	}

	logger.Infof("✓ Upload successful!")
	logger.Infof("  File: %s", filepath.Base(filePath))
	logger.Infof("  Size: %d bytes", info.Size())
	logger.Infof("  CID: %s", result.CID)
	logger.Infof("  Pinned: %v", addOpts.Pin)

	if addOpts.NoCopy {
		logger.Info("  Mode: nocopy (filestore)")
	}

	return nil
}

// publishAnnouncementViaIPFS publishes a PubSub announcement via embedded IPFS node's PubSub
func publishAnnouncementViaIPFS(ctx context.Context, client ipfs.Client, topic string, ipns string, collectionSize int, version int) error {
	// Only works with embedded IPFS client
	embeddedClient, ok := client.(*ipfs.EmbeddedClient)
	if !ok {
		return fmt.Errorf("PubSub only supported with embedded IPFS mode")
	}

	// Create announcement message
	msg := pubsub.NewAnnouncementMessage(version, ipns, collectionSize, time.Now().Unix())

	// Load keys for signing
	keyMgr := keys.New(filepath.Join(getBaseDir(), "keys"))
	if err := keyMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize keys: %w", err)
	}

	// Sign the message
	if err := msg.Sign(keyMgr.GetPrivateKey()); err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	// Convert to JSON
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Publish via embedded IPFS node's PubSub
	if err := embeddedClient.PublishToPubSub(ctx, topic, data); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	return nil
}

// initPubSubNode initializes a standalone libp2p PubSub node for external IPFS mode
func initPubSubNode(cfg *config.Config) (*pubsub.Node, error) {
	pubsubCfg := &pubsub.Config{
		Topic:          cfg.Pubsub.Topic,
		ListenPort:     cfg.Pubsub.ListenPort,
		BootstrapPeers: cfg.Pubsub.BootstrapPeers,
	}

	node, err := pubsub.NewNode(pubsubCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create PubSub node: %w", err)
	}

	if err := node.Start(pubsubCfg); err != nil {
		return nil, fmt.Errorf("failed to start PubSub node: %w", err)
	}

	return node, nil
}

// publishAnnouncementViaStandalone publishes a PubSub announcement via standalone libp2p node
func publishAnnouncementViaStandalone(ctx context.Context, node *pubsub.Node, topic string, ipns string, collectionSize int, version int) error {
	// Create announcement message
	msg := pubsub.NewAnnouncementMessage(version, ipns, collectionSize, time.Now().Unix())

	// Load keys for signing
	keyMgr := keys.New(filepath.Join(getBaseDir(), "keys"))
	if err := keyMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize keys: %w", err)
	}

	// Sign the message
	if err := msg.Sign(keyMgr.GetPrivateKey()); err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	// Convert to JSON
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Publish via standalone PubSub node
	if err := node.Publish(data); err != nil {
		return fmt.Errorf("failed to publish: %w", err)
	}

	log := logger.Get()
	log.Debugf("Published announcement to topic %s (peers: %d)", topic, node.GetTopicPeerCount())

	return nil
}

// runPeriodicAnnouncementsStandalone runs periodic PubSub announcements for external IPFS mode
func runPeriodicAnnouncementsStandalone(cfg *config.Config, node *pubsub.Node) {
	log := logger.Get()
	ticker := time.NewTicker(time.Duration(cfg.Pubsub.AnnounceInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Load current state
		stateManager := state.New(filepath.Join(getBaseDir(), "state.json"))
		if err := stateManager.Load(); err != nil {
			log.Debugf("No state to announce: %v", err)
			continue
		}

		ipns := stateManager.GetIPNS()
		if ipns == "" {
			log.Debug("No IPNS to announce yet")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := publishAnnouncementViaStandalone(ctx, node, cfg.Pubsub.Topic, ipns, len(stateManager.GetAllFiles()), stateManager.GetVersion())
		cancel()

		if err != nil {
			log.Warnf("Failed to publish periodic announcement: %v", err)
		} else {
			log.Infof("✓ Periodic announcement published (version %d, peers: %d)", stateManager.GetVersion(), node.GetTopicPeerCount())
		}
	}
}

// runPeriodicAnnouncementsEmbedded runs periodic PubSub announcements for embedded IPFS mode
func runPeriodicAnnouncementsEmbedded(cfg *config.Config, client ipfs.Client) {
	log := logger.Get()
	ticker := time.NewTicker(time.Duration(cfg.Pubsub.AnnounceInterval) * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Load current state
		stateManager := state.New(filepath.Join(getBaseDir(), "state.json"))
		if err := stateManager.Load(); err != nil {
			log.Debugf("No state to announce: %v", err)
			continue
		}

		ipns := stateManager.GetIPNS()
		if ipns == "" {
			log.Debug("No IPNS to announce yet")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := publishAnnouncementViaIPFS(ctx, client, cfg.Pubsub.Topic, ipns, len(stateManager.GetAllFiles()), stateManager.GetVersion())
		cancel()

		if err != nil {
			log.Warnf("Failed to publish periodic announcement: %v", err)
		} else {
			log.Infof("✓ Periodic announcement published (version %d)", stateManager.GetVersion())
		}
	}
}

// testIPNSOperations tests IPNS publish and resolve
func testIPNSOperations(client ipfs.Client) error {
	logger.Info("Testing IPNS operations...")

	// Create a test string and upload it
	testContent := "Hello from IPFS Publisher - Test IPNS\n"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	logger.Info("1. Uploading test content to IPFS...")
	result, err := client.Add(ctx, strings.NewReader(testContent), "test-ipns.txt", ipfs.AddOptions{
		Pin:       true,
		RawLeaves: true,
	})
	if err != nil {
		return fmt.Errorf("failed to add test content: %w", err)
	}
	logger.Infof("   CID: %s", result.CID)

	// Publish to IPNS
	logger.Info("2. Publishing to IPNS...")
	ipnsResult, err := client.PublishIPNS(ctx, result.CID, ipfs.IPNSPublishOptions{
		Lifetime: "24h",
	})
	if err != nil {
		return fmt.Errorf("failed to publish to IPNS: %w", err)
	}
	logger.Infof("   IPNS Name: %s", ipnsResult.Name)
	logger.Infof("   Points to: %s", ipnsResult.Value)

	// Resolve IPNS
	logger.Info("3. Resolving IPNS name...")
	resolvedPath, err := client.ResolveIPNS(ctx, ipnsResult.Name)
	if err != nil {
		return fmt.Errorf("failed to resolve IPNS: %w", err)
	}
	logger.Infof("   Resolved to: %s", resolvedPath)

	// Verify
	if strings.Contains(resolvedPath, result.CID) {
		logger.Info("✓ IPNS test successful!")
	} else {
		logger.Warnf("IPNS resolved to different CID: expected %s in %s", result.CID, resolvedPath)
	}

	return nil
}

func testPubSubOperations(cfg *config.Config) error {
	logger := logger.Get()
	ctx := context.Background()
	_ = ctx

	logger.Info("Testing PubSub operations...")

	// Generate test keypair
	logger.Info("1. Generating Ed25519 keypair...")
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("failed to generate keypair: %w", err)
	}
	logger.Infof("   ✓ Keypair generated (public key: %s...)", base64.StdEncoding.EncodeToString(publicKey)[:32])

	// Create PubSub node
	logger.Info("2. Creating PubSub node...")

	nodeCfg := &pubsub.Config{
		Topic:          cfg.Pubsub.Topic,
		BootstrapPeers: cfg.Pubsub.BootstrapPeers,
	}

	node, err := pubsub.NewNode(nodeCfg)
	if err != nil {
		return fmt.Errorf("failed to create PubSub node: %w", err)
	}

	if err := node.Start(nodeCfg); err != nil {
		return fmt.Errorf("failed to start PubSub node: %w", err)
	}
	defer node.Stop()
	logger.Info("   ✓ PubSub node started")

	// Wait for peer discovery
	logger.Info("3. Waiting for peer discovery...")
	time.Sleep(5 * time.Second)
	peerCount := node.GetPeerCount()
	topicPeerCount := node.GetTopicPeerCount()
	logger.Infof("   Connected to %d peers (%d on topic)", peerCount, topicPeerCount)

	// Create and publish test message
	logger.Info("4. Creating test announcement message...")
	msg := pubsub.NewAnnouncementMessage(
		1,                                   // version
		"k51qzi5uqu5dh9ihj8p0dxgzm4jw8m...", // test IPNS
		10,                                  // collection size
		time.Now().Unix(),
	)

	if err := msg.Sign(privateKey); err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}
	logger.Info("   ✓ Message created and signed")

	// Verify signature
	logger.Info("5. Verifying signature...")
	if err := msg.Verify(); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}
	logger.Infof("   ✓ Signature verified with public key: %s...", base64.StdEncoding.EncodeToString(publicKey)[:32])

	// Publish message
	logger.Info("6. Publishing message to PubSub...")
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	if err := node.Publish(data); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	logger.Infof("   ✓ Message published to topic: %s", cfg.Pubsub.Topic)

	// Display message content
	logger.Info("7. Message content:")
	logger.Infof("   Version: %d", msg.Version)
	logger.Infof("   IPNS: %s", msg.IPNS)
	logger.Infof("   Collection Size: %d", msg.CollectionSize)
	logger.Infof("   Timestamp: %d", msg.Timestamp)

	logger.Info("✓ PubSub test successful!")
	return nil
}

func runScan(cfg *config.Config, ipfsClient ipfs.Client, dryRun bool) error {
	logger := logger.Get()
	ctx := context.Background()

	// Initialize scanner
	scan := scanner.New(cfg.Directories, cfg.Extensions)
	logger.Infof("Scanning directories: %v", cfg.Directories)
	logger.Infof("Looking for extensions: %v", cfg.Extensions)

	// Scan for files
	files, err := scan.Scan()
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(files) == 0 {
		logger.Warn("No files found matching criteria")
		return nil
	}

	logger.Infof("Found %d files", len(files))

	if dryRun {
		logger.Info("Dry-run mode: listing files without uploading")
		for i, file := range files {
			logger.Infof("[%d] %s (%d bytes)", i+1, file.Path, file.Size)
		}
		return nil
	}

	// Initialize state manager
	stateManager := state.New(filepath.Join(getBaseDir(), "state.json"))
	if err := stateManager.Load(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Initialize index manager
	indexPath := filepath.Join(getBaseDir(), "collection.ndjson")
	indexMgr := index.New(indexPath)
	if err := indexMgr.Load(); err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	// Create progress bar
	var bar *progressbar.ProgressBar
	if len(files) > 10 {
		bar = progressbar.NewOptions(len(files),
			progressbar.OptionSetDescription("Uploading files"),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowCount(),
			progressbar.OptionSetPredictTime(true),
			progressbar.OptionFullWidth(),
			progressbar.OptionSetRenderBlankState(true),
		)
	}

	// Process files
	processedCount := 0
	skippedCount := 0
	errorCount := 0

	for _, file := range files {
		// Check if file needs processing
		fileState, exists := stateManager.GetFile(file.Path)
		if exists && fileState.ModTime == file.ModTime && fileState.Size == file.Size {
			// File unchanged, skip
			skippedCount++
			if bar != nil {
				bar.Add(1)
			}
			continue
		}

		// Upload file to IPFS
		logger.Infof("Uploading: %s", file.Name)

		// Open file
		f, err := os.Open(file.Path)
		if err != nil {
			logger.Errorf("Failed to open %s: %v", file.Path, err)
			errorCount++
			if bar != nil {
				bar.Add(1)
			}
			continue
		}

		// Determine add options based on mode
		addOpts := ipfs.AddOptions{
			Pin:       true,
			RawLeaves: true,
		}

		if cfg.IPFS.Mode == config.IPFSModeExternal {
			if pin, ok := cfg.IPFS.External.Options["pin"].(bool); ok {
				addOpts.Pin = pin
			}
			if rawLeaves, ok := cfg.IPFS.External.Options["raw_leaves"].(bool); ok {
				addOpts.RawLeaves = rawLeaves
			}
			if chunker, ok := cfg.IPFS.External.Options["chunker"].(string); ok {
				addOpts.Chunker = chunker
			}
		} else {
			if pin, ok := cfg.IPFS.Embedded.Options["pin"].(bool); ok {
				addOpts.Pin = pin
			}
			if rawLeaves, ok := cfg.IPFS.Embedded.Options["raw_leaves"].(bool); ok {
				addOpts.RawLeaves = rawLeaves
			}
			if chunker, ok := cfg.IPFS.Embedded.Options["chunker"].(string); ok {
				addOpts.Chunker = chunker
			}
		}

		result, err := ipfsClient.Add(ctx, f, file.Name, addOpts)
		f.Close()

		if err != nil {
			logger.Errorf("Failed to upload %s: %v", file.Path, err)
			errorCount++
			if bar != nil {
				bar.Add(1)
			}
			continue
		}

		logger.Infof("   ✓ CID: %s", result.CID)

		// Update index
		if exists {
			indexMgr.Update(file.Name, result.CID)
		} else {
			record := indexMgr.Add(file.Name, result.CID, file.Extension)
			// Update state with index ID
			stateManager.SetFile(file.Path, &state.FileState{
				CID:     result.CID,
				ModTime: file.ModTime,
				Size:    file.Size,
				IndexID: record.ID,
			})
		}

		processedCount++
		if bar != nil {
			bar.Add(1)
		}
	}

	if bar != nil {
		bar.Finish()
	}

	logger.Infof("Processing complete: %d uploaded, %d skipped, %d errors", processedCount, skippedCount, errorCount)

	// Always update IPNS and publish announcements (even if no files changed)
	// Save and upload index if needed
	var indexCID string
	if processedCount > 0 {
		if err := indexMgr.Save(); err != nil {
			return fmt.Errorf("failed to save index: %w", err)
		}
		logger.Info("Index saved")

		// Upload index to IPFS
		indexFile, err := os.Open(indexMgr.GetPath())
		if err != nil {
			return fmt.Errorf("failed to open index file: %w", err)
		}

		indexResult, err := ipfsClient.Add(ctx, indexFile, "collection.ndjson", ipfs.AddOptions{
			Pin: true,
		})
		indexFile.Close()

		if err != nil {
			return fmt.Errorf("failed to upload index: %w", err)
		}

		logger.Infof("Index uploaded to IPFS: %s", indexResult.CID)
		indexCID = indexResult.CID
		stateManager.SetLastIndexCID(indexResult.CID)
		stateManager.IncrementVersion()
	} else {
		// No changes, use existing index CID
		indexCID = stateManager.GetLastIndexCID()
		if indexCID == "" {
			logger.Warn("No index CID available, skipping IPNS update")
			return nil
		}
		logger.Infof("No file changes, using existing index CID: %s", indexCID)
	}

	// Initialize key manager
	keyMgr := keys.New(filepath.Join(getBaseDir(), "keys"))
	if err := keyMgr.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize keys: %w", err)
	}

	// Publish to IPNS (with longer timeout for DHT propagation)
	logger.Info("Publishing to IPNS...")
	ipnsCtx, ipnsCancel := context.WithTimeout(ctx, 60*time.Second)
	defer ipnsCancel()

	ipnsResult, err := ipfsClient.PublishIPNS(ipnsCtx, indexCID, ipfs.IPNSPublishOptions{
		Key:          "self",
		Lifetime:     "24h",
		TTL:          "1h",
		AllowOffline: false, // Try to publish to DHT
	})
	if err != nil {
		logger.Errorf("Failed to publish IPNS: %v", err)
		logger.Info("   Retrying with offline mode...")

		// Retry with offline mode
		ipnsCtx2, ipnsCancel2 := context.WithTimeout(ctx, 10*time.Second)
		defer ipnsCancel2()

		ipnsResult, err = ipfsClient.PublishIPNS(ipnsCtx2, indexCID, ipfs.IPNSPublishOptions{
			Key:          "self",
			Lifetime:     "24h",
			TTL:          "1h",
			AllowOffline: true,
		})
		if err != nil {
			logger.Errorf("Failed to publish IPNS even in offline mode: %v", err)
			logger.Warn("   Skipping IPNS and PubSub announcement")
			return nil
		}
	}

	logger.Infof("✓ Published to IPNS: %s", ipnsResult.Name)
	logger.Infof("   Points to: %s", ipnsResult.Value)
	stateManager.SetIPNS(ipnsResult.Name)

	// Note: PubSub announcement will be published by the periodic announcer
	// The periodic goroutine will pick up the new IPNS and announce it

	// Save state
	if err := stateManager.Save(); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}
	logger.Info("State saved")

	return nil
}

// showNodePeerInfo displays the peer addresses for the IPFS node and PubSub node
func showNodePeerInfo(cfg *config.Config) error {
	ctx := context.Background()

	// Initialize IPFS client
	ipfsClient, err := createIPFSClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize IPFS client: %w", err)
	}
	defer ipfsClient.Close()

	fmt.Println("IPFS Node Information:")
	fmt.Printf("Mode: %s\n\n", cfg.IPFS.Mode)

	// For external node
	if cfg.IPFS.Mode == config.IPFSModeExternal {
		if extClient, ok := ipfsClient.(*ipfs.ExternalClient); ok {
			peerID, err := extClient.GetID()
			if err != nil {
				return fmt.Errorf("failed to get peer ID: %w", err)
			}
			fmt.Printf("IPFS Peer ID: %s\n", peerID)
			fmt.Printf("API URL: %s\n", cfg.IPFS.External.APIURL)
		}

		// Show standalone PubSub node info if enabled
		if cfg.Pubsub.Enabled {
			fmt.Println("\n=== Standalone PubSub Node (External Mode) ===")
			fmt.Println("Initializing standalone PubSub node...")

			pubsubNode, err := initPubSubNode(cfg)
			if err != nil {
				return fmt.Errorf("failed to initialize PubSub node: %w", err)
			}
			defer pubsubNode.Stop()

			// Get PubSub node info
			peerID := pubsubNode.GetPeerID()
			addrs := pubsubNode.GetListenAddresses()

			fmt.Printf("\nPubSub Peer ID: %s\n", peerID)
			fmt.Printf("Topic: %s\n", cfg.Pubsub.Topic)
			fmt.Printf("Connected peers: %d\n", pubsubNode.GetPeerCount())
			fmt.Printf("Topic peers: %d\n", pubsubNode.GetTopicPeerCount())

			if len(addrs) > 0 {
				fmt.Println("\nListen addresses:")
				for _, addr := range addrs {
					fmt.Printf("  %s\n", addr)
				}

				fmt.Println("\n=== To receive PubSub messages from this node ===")
				fmt.Println("Run this command from your IPFS node:")

				// Show the first TCP address
				for _, addr := range addrs {
					if strings.Contains(addr, "/tcp/") && !strings.Contains(addr, "/127.0.0.1/") {
						fmt.Printf("\n  ipfs swarm connect %s\n", addr)
						break
					}
				}

				// Also show localhost address
				for _, addr := range addrs {
					if strings.Contains(addr, "/tcp/") && strings.Contains(addr, "/127.0.0.1/") {
						fmt.Printf("\n  # Or if on the same machine:\n  ipfs swarm connect %s\n", addr)
						break
					}
				}

				fmt.Println("\nThen subscribe to announcements:")
				fmt.Printf("  ipfs pubsub sub %s\n", cfg.Pubsub.Topic)
			}
		}

		return nil
	}

	// For embedded node, show listen addresses
	if cfg.IPFS.Mode == config.IPFSModeEmbedded {
		if embeddedClient, ok := ipfsClient.(*ipfs.EmbeddedClient); ok {
			// Get peer addresses
			addrs, err := embeddedClient.GetPeerAddresses(ctx)
			if err != nil {
				return fmt.Errorf("failed to get peer addresses: %w", err)
			}

			// Extract peer ID from first address
			var peerID string
			if len(addrs) > 0 {
				parts := strings.Split(addrs[0], "/p2p/")
				if len(parts) == 2 {
					peerID = parts[1]
					fmt.Printf("Peer ID: %s\n\n", peerID)
				}
			}

			fmt.Println("Listen addresses:")
			for _, addr := range addrs {
				fmt.Printf("  %s\n", addr)
			}

			if len(addrs) > 0 {
				fmt.Println("\n=== To receive PubSub messages from this node ===")
				fmt.Println("Run this command from your external IPFS node:")

				// Show the first TCP address (usually most useful)
				for _, addr := range addrs {
					if strings.Contains(addr, "/tcp/") && !strings.Contains(addr, "/127.0.0.1/") {
						fmt.Printf("\n  ipfs swarm connect %s\n", addr)
						break
					}
				}

				// Also show localhost address
				for _, addr := range addrs {
					if strings.Contains(addr, "/tcp/") && strings.Contains(addr, "/127.0.0.1/") {
						fmt.Printf("\n  # Or if on the same machine:\n  ipfs swarm connect %s\n", addr)
						break
					}
				}

				fmt.Println("\nAfter connecting, subscribe to announcements:")
				fmt.Printf("  ipfs pubsub sub %s\n", cfg.Pubsub.Topic)
			}
		}
	}

	return nil
}
