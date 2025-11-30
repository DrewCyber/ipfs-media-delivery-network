package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/atregu/ipfs-indexer/internal/database"
	"github.com/atregu/ipfs-indexer/internal/ipfs"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/sirupsen/logrus"
)

// Message represents a PubSub message announcing a collection
type Message struct {
	Version        int    `json:"version"`
	IPNS           string `json:"ipns"`
	PublicKey      string `json:"publicKey"`
	CollectionSize *int   `json:"collectionSize,omitempty"`
	Timestamp      int64  `json:"timestamp"`
	Signature      string `json:"signature"`
}

// Listener handles PubSub subscriptions and message processing
type Listener struct {
	ipfsClient *ipfs.Client
	db         *database.DB
	topic      string
	log        *logrus.Logger
	ctx        context.Context
	cancel     context.CancelFunc
	sub        *pubsub.Subscription
}

// NewListener creates a new PubSub listener
func NewListener(ipfsClient *ipfs.Client, db *database.DB, topic string, log *logrus.Logger) *Listener {
	ctx, cancel := context.WithCancel(context.Background())
	return &Listener{
		ipfsClient: ipfsClient,
		db:         db,
		topic:      topic,
		log:        log,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start subscribes to the PubSub topic and begins processing messages
func (l *Listener) Start() error {
	l.log.Infof("Subscribing to PubSub topic: %s", l.topic)

	sub, err := l.ipfsClient.Subscribe(l.ctx, l.topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}
	l.sub = sub

	l.log.Infof("Successfully subscribed to topic: %s", l.topic)

	// Start message processing in a goroutine
	go l.processMessages()

	return nil
}

// processMessages continuously processes incoming PubSub messages
func (l *Listener) processMessages() {
	l.log.Info("Started processing PubSub messages")

	for {
		select {
		case <-l.ctx.Done():
			l.log.Info("Stopping PubSub message processing")
			return
		default:
			msg, err := l.sub.Next(l.ctx)
			if err != nil {
				if l.ctx.Err() != nil {
					// Context cancelled, exit gracefully
					return
				}
				l.log.Errorf("Error receiving message: %v", err)
				continue
			}

			// Process the message
			if err := l.handleMessage(msg); err != nil {
				l.log.Errorf("Error handling message: %v", err)
			}
		}
	}
}

// handleMessage processes a single PubSub message
func (l *Listener) handleMessage(msg *pubsub.Message) error {
	// Extract sender peer ID (host)
	senderID := msg.ReceivedFrom.String()
	l.log.Debugf("Received message from peer: %s", senderID)

	// Parse the message
	var collMsg Message
	if err := json.Unmarshal(msg.Data, &collMsg); err != nil {
		l.log.Warnf("Failed to parse message: %v", err)
		return nil // Don't return error, just skip this message
	}

	// Validate the message
	if err := l.validateMessage(&collMsg); err != nil {
		l.log.Warnf("Invalid message: %v", err)
		return nil // Don't return error, just skip this message
	}

	l.log.Infof("Valid collection announcement received: IPNS=%s, Version=%d, Size=%v, Timestamp=%d",
		collMsg.IPNS, collMsg.Version, collMsg.CollectionSize, collMsg.Timestamp)

	// Store in database
	if err := l.storeAnnouncement(senderID, &collMsg); err != nil {
		return fmt.Errorf("failed to store announcement: %w", err)
	}

	return nil
}

// validateMessage performs basic validation on the message
func (l *Listener) validateMessage(msg *Message) error {
	// Check required fields
	if msg.Version == 0 {
		return fmt.Errorf("missing required field: version")
	}

	if msg.IPNS == "" {
		return fmt.Errorf("missing required field: ipns")
	}

	if msg.PublicKey == "" {
		return fmt.Errorf("missing required field: publicKey")
	}

	if msg.Timestamp == 0 {
		return fmt.Errorf("missing required field: timestamp")
	}

	// Validate IPNS format (should start with "k2k4r8")
	if !strings.HasPrefix(msg.IPNS, "k2k4r8") {
		return fmt.Errorf("invalid IPNS format: must start with k2k4r8")
	}

	return nil
}

// storeAnnouncement stores the announcement in the database
func (l *Listener) storeAnnouncement(hostPublicKey string, msg *Message) error {
	// Create or get host
	host, err := l.db.CreateOrGetHost(hostPublicKey)
	if err != nil {
		return fmt.Errorf("failed to create/get host: %w", err)
	}

	// Create or get publisher
	publisher, err := l.db.CreateOrGetPublisher(msg.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to create/get publisher: %w", err)
	}

	// Create collection
	collection, err := l.db.CreateCollection(
		host.ID,
		publisher.ID,
		msg.Version,
		msg.IPNS,
		msg.CollectionSize,
		msg.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	l.log.Infof("Stored collection announcement: ID=%d, IPNS=%s, Status=pending", collection.ID, msg.IPNS)

	return nil
}

// Stop gracefully stops the PubSub listener
func (l *Listener) Stop() error {
	l.log.Info("Stopping PubSub listener...")

	// Cancel context
	if l.cancel != nil {
		l.cancel()
	}

	// Unsubscribe
	if l.sub != nil {
		l.sub.Cancel()
	}

	l.log.Info("PubSub listener stopped")
	return nil
}
