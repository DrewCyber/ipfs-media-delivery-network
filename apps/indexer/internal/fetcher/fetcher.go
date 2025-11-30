package fetcher

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/atregu/ipfs-indexer/internal/config"
	"github.com/atregu/ipfs-indexer/internal/database"
	"github.com/atregu/ipfs-indexer/internal/ipfs"
	"github.com/atregu/ipfs-indexer/internal/parser"
	"github.com/sirupsen/logrus"
)

// Fetcher handles downloading collections from IPNS
type Fetcher struct {
	ipfsClient *ipfs.Client
	db         *database.DB
	parser     *parser.Parser
	cfg        *config.FetcherConfig
	log        *logrus.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	semaphore  chan struct{}
}

// NewFetcher creates a new collection fetcher
func NewFetcher(ipfsClient *ipfs.Client, db *database.DB, parser *parser.Parser, cfg *config.FetcherConfig, log *logrus.Logger) *Fetcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Fetcher{
		ipfsClient: ipfsClient,
		db:         db,
		parser:     parser,
		cfg:        cfg,
		log:        log,
		ctx:        ctx,
		cancel:     cancel,
		semaphore:  make(chan struct{}, cfg.ConcurrentDownloads),
	}
}

// Start begins the background fetcher goroutine
func (f *Fetcher) Start() error {
	f.log.Info("Starting collection fetcher...")

	// Start the background worker
	f.wg.Add(1)
	go f.worker()

	f.log.Info("Collection fetcher started")
	return nil
}

// worker is the main background goroutine that processes pending collections
func (f *Fetcher) worker() {
	defer f.wg.Done()

	ticker := time.NewTicker(time.Duration(f.cfg.RetryIntervalSeconds) * time.Second)
	defer ticker.Stop()

	// Process immediately on start
	f.processPendingCollections()

	for {
		select {
		case <-f.ctx.Done():
			f.log.Info("Stopping collection fetcher worker...")
			return
		case <-ticker.C:
			f.processPendingCollections()
		}
	}
}

// processPendingCollections fetches all pending collections
func (f *Fetcher) processPendingCollections() {
	collections, err := f.db.GetPendingCollections(f.cfg.RetryAttempts)
	if err != nil {
		f.log.Errorf("Failed to get pending collections: %v", err)
		return
	}

	if len(collections) == 0 {
		f.log.Debug("No pending collections to process")
		return
	}

	f.log.Infof("Processing %d pending collections...", len(collections))

	for _, collection := range collections {
		// Check if we should retry (check last retry time)
		if collection.LastRetryAt != nil && collection.RetryCount > 0 {
			// Don't retry too soon
			continue
		}

		// Use semaphore to limit concurrent downloads
		select {
		case <-f.ctx.Done():
			return
		case f.semaphore <- struct{}{}:
			f.wg.Add(1)
			go f.fetchCollection(collection)
		}
	}
}

// fetchCollection downloads and processes a single collection
func (f *Fetcher) fetchCollection(collection *database.Collection) {
	defer f.wg.Done()
	defer func() { <-f.semaphore }()

	f.log.Infof("Fetching collection ID=%d, IPNS=%s (attempt %d/%d)",
		collection.ID, collection.IPNS, collection.RetryCount+1, f.cfg.RetryAttempts)

	// Create a timeout context for the fetch operation
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Minute)
	defer cancel()

	// Step 1: Resolve IPNS to CID
	cid, err := f.ipfsClient.ResolveIPNS(ctx, collection.IPNS)
	if err != nil {
		f.handleFetchError(collection, fmt.Errorf("failed to resolve IPNS: %w", err))
		return
	}

	f.log.Infof("Resolved IPNS %s to CID: %s", collection.IPNS, cid)

	// Step 2: Download the file content
	reader, err := f.ipfsClient.Cat(ctx, cid)
	if err != nil {
		f.handleFetchError(collection, fmt.Errorf("failed to fetch CID %s: %w", cid, err))
		return
	}
	defer reader.Close()

	// Step 3: Read the content
	content, err := io.ReadAll(reader)
	if err != nil {
		f.handleFetchError(collection, fmt.Errorf("failed to read content: %w", err))
		return
	}

	f.log.Infof("Downloaded collection ID=%d, size=%d bytes", collection.ID, len(content))

	// Step 4: Parse and store the collection
	count, err := f.parser.ParseAndStore(collection, content)
	if err != nil {
		f.handleFetchError(collection, fmt.Errorf("failed to parse collection: %w", err))
		return
	}

	// Step 5: Update collection status to downloaded
	size := len(content)
	if err := f.db.UpdateCollectionStatus(collection.ID, "downloaded", &size); err != nil {
		f.log.Errorf("Failed to update collection status: %v", err)
		return
	}

	f.log.Infof("Successfully processed collection ID=%d, indexed %d items", collection.ID, count)
}

// handleFetchError handles errors during fetching, implementing retry logic
func (f *Fetcher) handleFetchError(collection *database.Collection, err error) {
	f.log.Warnf("Error fetching collection ID=%d: %v", collection.ID, err)

	// Increment retry count
	if err := f.db.IncrementRetryCount(collection.ID); err != nil {
		f.log.Errorf("Failed to increment retry count: %v", err)
		return
	}

	// Check if we've reached max retries
	if collection.RetryCount+1 >= f.cfg.RetryAttempts {
		// Mark as failed
		if err := f.db.UpdateCollectionStatus(collection.ID, "failed", nil); err != nil {
			f.log.Errorf("Failed to update collection status to failed: %v", err)
		}
		f.log.Warnf("Collection ID=%d marked as failed after %d attempts", collection.ID, collection.RetryCount+1)
	}
}

// Stop gracefully stops the fetcher
func (f *Fetcher) Stop() error {
	f.log.Info("Stopping collection fetcher...")

	// Cancel context
	if f.cancel != nil {
		f.cancel()
	}

	// Wait for all goroutines to finish
	f.wg.Wait()

	f.log.Info("Collection fetcher stopped")
	return nil
}
