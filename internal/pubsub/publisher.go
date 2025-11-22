package pubsub

import (
	"crypto/ed25519"
	"fmt"
	"sync"
	"time"

	"github.com/atregu/ipfs-publisher/internal/logger"
)

// Publisher handles publishing announcements to PubSub
type Publisher struct {
	node             *Node
	privateKey       ed25519.PrivateKey
	currentVersion   int
	currentIPNS      string
	collectionSize   int
	lastTimestamp    int64
	announceInterval time.Duration
	ticker           *time.Ticker
	stopChan         chan struct{}
	mu               sync.RWMutex
	started          bool
}

// PublisherConfig holds publisher configuration
type PublisherConfig struct {
	AnnounceInterval time.Duration // How often to repeat announcements
}

// NewPublisher creates a new publisher
func NewPublisher(node *Node, privateKey ed25519.PrivateKey, cfg *PublisherConfig) *Publisher {
	return &Publisher{
		node:             node,
		privateKey:       privateKey,
		announceInterval: cfg.AnnounceInterval,
		stopChan:         make(chan struct{}),
	}
}

// Start starts the periodic announcement loop
func (p *Publisher) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return fmt.Errorf("publisher already started")
	}

	log := logger.Get()
	log.Infof("Starting PubSub publisher with interval: %v", p.announceInterval)

	p.ticker = time.NewTicker(p.announceInterval)
	p.started = true

	go p.announceLoop()

	return nil
}

// announceLoop periodically publishes announcements
func (p *Publisher) announceLoop() {
	log := logger.Get()

	for {
		select {
		case <-p.ticker.C:
			p.mu.RLock()
			// Announce if we have either IPNS or just a version/collection
			if p.currentIPNS != "" || p.currentVersion > 0 {
				log.Debug("Periodic announcement triggered")
				if err := p.publishCurrent(); err != nil {
					log.Errorf("Failed to publish periodic announcement: %v", err)
				}
			}
			p.mu.RUnlock()

		case <-p.stopChan:
			log.Debug("Announcement loop stopped")
			return
		}
	}
}

// Announce publishes a new announcement (increments version)
func (p *Publisher) Announce(ipns string, collectionSize int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	log := logger.Get()

	// Increment version for new announcement
	p.currentVersion++
	p.currentIPNS = ipns
	p.collectionSize = collectionSize
	p.lastTimestamp = time.Now().Unix()

	log.Infof("Publishing announcement: version=%d, IPNS=%s, size=%d",
		p.currentVersion, ipns, collectionSize)

	return p.publishCurrentLocked()
}

// publishCurrent publishes the current state without changing version
func (p *Publisher) publishCurrent() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.publishCurrentLocked()
}

// publishCurrentLocked publishes without locking (caller must hold lock)
func (p *Publisher) publishCurrentLocked() error {
	// Require IPNS before publishing
	if p.currentVersion == 0 {
		return fmt.Errorf("no announcement to publish (version 0)")
	}
	if p.currentIPNS == "" {
		return fmt.Errorf("no IPNS to publish")
	}

	log := logger.Get()

	// Create message
	msg := NewAnnouncementMessage(
		p.currentVersion,
		p.currentIPNS,
		p.collectionSize,
		p.lastTimestamp,
	)

	// Sign message
	if err := msg.Sign(p.privateKey); err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	// Convert to JSON
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Publish to PubSub
	if err := p.node.Publish(data); err != nil {
		return fmt.Errorf("failed to publish to PubSub: %w", err)
	}

	peerCount := p.node.GetTopicPeerCount()
	log.Infof("✓ Published announcement (version %d) to %d peers on topic",
		p.currentVersion, peerCount)

	return nil
}

// GetCurrentVersion returns the current version number
func (p *Publisher) GetCurrentVersion() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentVersion
}

// GetCurrentIPNS returns the current IPNS hash
func (p *Publisher) GetCurrentIPNS() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentIPNS
}

// Stop stops the publisher
func (p *Publisher) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return nil
	}

	log := logger.Get()
	log.Info("Stopping PubSub publisher...")

	if p.ticker != nil {
		p.ticker.Stop()
	}

	close(p.stopChan)
	p.started = false

	log.Info("PubSub publisher stopped")
	return nil
}
