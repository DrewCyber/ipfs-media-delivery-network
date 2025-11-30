package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/atregu/ipfs-indexer/internal/config"
	"github.com/atregu/ipfs-indexer/internal/database"
	"github.com/atregu/ipfs-indexer/internal/fetcher"
	"github.com/atregu/ipfs-indexer/internal/ipfs"
	"github.com/atregu/ipfs-indexer/internal/logger"
	"github.com/atregu/ipfs-indexer/internal/parser"
	"github.com/atregu/ipfs-indexer/internal/pubsub"
)

var (
	configPath = flag.String("config", "config.yaml", "Path to configuration file")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.Init(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output, cfg.Logging.FilePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	log := logger.Get()
	log.Info("Starting IPFS Indexer...")

	// Initialize database
	log.Info("Initializing database...")
	db, err := database.New(cfg.Database.Path, log)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize IPFS client
	log.Info("Initializing IPFS client...")
	ipfsClient, err := ipfs.NewClient(&cfg.IPFS.Embedded)
	if err != nil {
		log.Fatalf("Failed to create IPFS client: %v", err)
	}

	// Start IPFS node
	if err := ipfsClient.Start(); err != nil {
		log.Fatalf("Failed to start IPFS node: %v", err)
	}
	defer ipfsClient.Close()

	// Initialize parser
	contentParser := parser.NewParser(db, log)

	// Initialize fetcher
	log.Info("Initializing collection fetcher...")
	collectionFetcher := fetcher.NewFetcher(ipfsClient, db, contentParser, &cfg.Fetcher, log)
	if err := collectionFetcher.Start(); err != nil {
		log.Fatalf("Failed to start fetcher: %v", err)
	}
	defer collectionFetcher.Stop()

	// Initialize PubSub listener
	log.Info("Initializing PubSub listener...")
	pubsubListener := pubsub.NewListener(ipfsClient, db, cfg.Pubsub.Topic, log)
	if err := pubsubListener.Start(); err != nil {
		log.Fatalf("Failed to start PubSub listener: %v", err)
	}
	defer pubsubListener.Stop()

	log.Info("IPFS Indexer is running. Press Ctrl+C to stop.")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	<-sigChan
	log.Info("Received shutdown signal, gracefully shutting down...")

	// Graceful shutdown is handled by defer statements above
	log.Info("Shutdown complete")
}
