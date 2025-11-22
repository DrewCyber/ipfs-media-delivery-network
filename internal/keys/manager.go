package keys

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/atregu/ipfs-publisher/internal/logger"
)

// Manager handles Ed25519 key pair management
type Manager struct {
	keysDir    string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

// New creates a new key manager
func New(keysDir string) *Manager {
	return &Manager{
		keysDir: expandPath(keysDir),
	}
}

// Initialize loads or generates keys
func (m *Manager) Initialize() error {
	log := logger.Get()

	// Create keys directory with secure permissions
	if err := os.MkdirAll(m.keysDir, 0700); err != nil {
		return fmt.Errorf("failed to create keys directory: %w", err)
	}

	privateKeyPath := filepath.Join(m.keysDir, "private.key")

	// Check if keys exist
	if _, err := os.Stat(privateKeyPath); err == nil {
		// Load existing keys
		log.Info("Loading existing IPNS keypair...")
		if err := m.loadKeys(); err != nil {
			return fmt.Errorf("failed to load keys: %w", err)
		}
		log.Info("✓ IPNS keypair loaded successfully")
		return nil
	}

	// Generate new keys
	log.Info("Generating new Ed25519 keypair for IPNS...")
	if err := m.generateKeys(); err != nil {
		return fmt.Errorf("failed to generate keys: %w", err)
	}

	// Save keys
	if err := m.saveKeys(); err != nil {
		return fmt.Errorf("failed to save keys: %w", err)
	}

	log.Info("✓ IPNS keypair generated and saved")
	return nil
}

// generateKeys generates a new Ed25519 key pair
func (m *Manager) generateKeys() error {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate Ed25519 key: %w", err)
	}

	m.privateKey = privateKey
	m.publicKey = publicKey
	return nil
}

// saveKeys saves keys to disk with secure permissions
func (m *Manager) saveKeys() error {
	privateKeyPath := filepath.Join(m.keysDir, "private.key")
	publicKeyPath := filepath.Join(m.keysDir, "public.key")

	// Save private key with 0600 permissions
	privateKeyHex := hex.EncodeToString(m.privateKey)
	if err := os.WriteFile(privateKeyPath, []byte(privateKeyHex), 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Save public key with 0644 permissions
	publicKeyHex := hex.EncodeToString(m.publicKey)
	if err := os.WriteFile(publicKeyPath, []byte(publicKeyHex), 0644); err != nil {
		return fmt.Errorf("failed to write public key: %w", err)
	}

	return nil
}

// loadKeys loads keys from disk
func (m *Manager) loadKeys() error {
	privateKeyPath := filepath.Join(m.keysDir, "private.key")
	publicKeyPath := filepath.Join(m.keysDir, "public.key")

	// Load private key
	privateKeyHex, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key: %w", err)
	}

	privateKey, err := hex.DecodeString(string(privateKeyHex))
	if err != nil {
		return fmt.Errorf("failed to decode private key: %w", err)
	}

	if len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key size: expected %d, got %d", ed25519.PrivateKeySize, len(privateKey))
	}

	m.privateKey = ed25519.PrivateKey(privateKey)

	// Load public key
	publicKeyHex, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
	}

	publicKey, err := hex.DecodeString(string(publicKeyHex))
	if err != nil {
		return fmt.Errorf("failed to decode public key: %w", err)
	}

	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(publicKey))
	}

	m.publicKey = ed25519.PublicKey(publicKey)

	return nil
}

// GetPrivateKey returns the private key
func (m *Manager) GetPrivateKey() ed25519.PrivateKey {
	return m.privateKey
}

// GetPublicKey returns the public key
func (m *Manager) GetPublicKey() ed25519.PublicKey {
	return m.publicKey
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
