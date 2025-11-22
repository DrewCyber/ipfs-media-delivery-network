package pubsub

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// AnnouncementMessage represents a collection announcement in PubSub
type AnnouncementMessage struct {
	Version        int    `json:"version"`        // Update counter
	IPNS           string `json:"ipns"`           // IPNS hash
	PublicKey      string `json:"publicKey"`      // Base64-encoded Ed25519 public key
	CollectionSize int    `json:"collectionSize"` // Number of files in collection
	Timestamp      int64  `json:"timestamp"`      // Unix timestamp
	Signature      string `json:"signature"`      // Base64-encoded signature
}

// NewAnnouncementMessage creates a new announcement message
func NewAnnouncementMessage(version int, ipns string, collectionSize int, timestamp int64) *AnnouncementMessage {
	return &AnnouncementMessage{
		Version:        version,
		IPNS:           ipns,
		CollectionSize: collectionSize,
		Timestamp:      timestamp,
	}
}

// Sign signs the message with the provided private key
func (m *AnnouncementMessage) Sign(privateKey ed25519.PrivateKey) error {
	// Extract public key from private key
	publicKey := privateKey.Public().(ed25519.PublicKey)
	m.PublicKey = base64.StdEncoding.EncodeToString(publicKey)

	// Create message without signature for signing
	data, err := m.getBytesForSigning()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Sign the data
	signature := ed25519.Sign(privateKey, data)
	m.Signature = base64.StdEncoding.EncodeToString(signature)

	return nil
}

// Verify verifies the message signature
func (m *AnnouncementMessage) Verify() error {
	// Decode public key
	publicKeyBytes, err := base64.StdEncoding.DecodeString(m.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to decode public key: %w", err)
	}

	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKeyBytes))
	}

	publicKey := ed25519.PublicKey(publicKeyBytes)

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Get message bytes for verification
	data, err := m.getBytesForSigning()
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Verify signature
	if !ed25519.Verify(publicKey, data, signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// getBytesForSigning returns the canonical JSON representation for signing
func (m *AnnouncementMessage) getBytesForSigning() ([]byte, error) {
	// Create a copy without signature
	msg := struct {
		Version        int    `json:"version"`
		IPNS           string `json:"ipns"`
		PublicKey      string `json:"publicKey"`
		CollectionSize int    `json:"collectionSize"`
		Timestamp      int64  `json:"timestamp"`
	}{
		Version:        m.Version,
		IPNS:           m.IPNS,
		PublicKey:      m.PublicKey,
		CollectionSize: m.CollectionSize,
		Timestamp:      m.Timestamp,
	}

	return json.Marshal(msg)
}

// ToJSON converts the message to JSON bytes
func (m *AnnouncementMessage) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON parses a message from JSON bytes
func FromJSON(data []byte) (*AnnouncementMessage, error) {
	var msg AnnouncementMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &msg, nil
}

// Validate validates the message fields
func (m *AnnouncementMessage) Validate() error {
	if m.Version < 1 {
		return fmt.Errorf("invalid version: must be >= 1")
	}

	if m.IPNS == "" {
		return fmt.Errorf("IPNS field is required")
	}

	if m.PublicKey == "" {
		return fmt.Errorf("publicKey field is required")
	}

	if m.CollectionSize < 0 {
		return fmt.Errorf("invalid collectionSize: must be >= 0")
	}

	if m.Timestamp <= 0 {
		return fmt.Errorf("invalid timestamp: must be > 0")
	}

	// Timestamp should not be in the far future (allow 1 hour drift)
	now := time.Now().Unix()
	if m.Timestamp > now+3600 {
		return fmt.Errorf("timestamp is too far in the future")
	}

	if m.Signature == "" {
		return fmt.Errorf("signature field is required")
	}

	return nil
}
