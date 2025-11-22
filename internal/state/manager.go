package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/atregu/ipfs-publisher/internal/logger"
)

// FileState represents the state of a single file
type FileState struct {
	CID     string `json:"cid"`
	ModTime int64  `json:"mtime"`
	Size    int64  `json:"size"`
	IndexID int    `json:"indexId"`
}

// State represents the application state
type State struct {
	Version      int                   `json:"version"`
	IPNS         string                `json:"ipns"`
	LastIndexCID string                `json:"lastIndexCID"`
	Files        map[string]*FileState `json:"files"`
	mu           sync.RWMutex          `json:"-"`
}

// Manager handles state persistence
type Manager struct {
	state *State
	path  string
}

// New creates a new state manager
func New(statePath string) *Manager {
	return &Manager{
		state: &State{
			Version: 0,
			Files:   make(map[string]*FileState),
		},
		path: expandPath(statePath),
	}
}

// Load loads state from disk
func (m *Manager) Load() error {
	log := logger.Get()

	// Create directory if it doesn't exist
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Check if state file exists
	if _, err := os.Stat(m.path); os.IsNotExist(err) {
		log.Info("State file does not exist, starting fresh")
		return nil
	}

	// Read state file
	data, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	// Parse JSON
	if err := json.Unmarshal(data, m.state); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	// Initialize Files map if nil
	if m.state.Files == nil {
		m.state.Files = make(map[string]*FileState)
	}

	log.Infof("Loaded state: version=%d, files=%d", m.state.Version, len(m.state.Files))
	return nil
}

// Save writes state to disk
func (m *Manager) Save() error {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	// Marshal to JSON
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write to temporary file
	tmpPath := m.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, m.path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// GetFile returns file state
func (m *Manager) GetFile(path string) (*FileState, bool) {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	fs, exists := m.state.Files[path]
	return fs, exists
}

// SetFile updates file state
func (m *Manager) SetFile(path string, fs *FileState) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.Files[path] = fs
}

// DeleteFile removes file from state
func (m *Manager) DeleteFile(path string) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	delete(m.state.Files, path)
}

// IncrementVersion increments and returns the new version
func (m *Manager) IncrementVersion() int {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.Version++
	return m.state.Version
}

// GetVersion returns current version
func (m *Manager) GetVersion() int {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	return m.state.Version
}

// SetIPNS sets the IPNS hash
func (m *Manager) SetIPNS(ipns string) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.IPNS = ipns
}

// GetIPNS returns the IPNS hash
func (m *Manager) GetIPNS() string {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	return m.state.IPNS
}

// SetLastIndexCID sets the last index CID
func (m *Manager) SetLastIndexCID(cid string) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	m.state.LastIndexCID = cid
}

// GetLastIndexCID returns the last index CID
func (m *Manager) GetLastIndexCID() string {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	return m.state.LastIndexCID
}

// GetAllFiles returns a copy of all file states
func (m *Manager) GetAllFiles() map[string]*FileState {
	m.state.mu.RLock()
	defer m.state.mu.RUnlock()

	files := make(map[string]*FileState, len(m.state.Files))
	for k, v := range m.state.Files {
		files[k] = v
	}
	return files
}

// expandPath expands ~ to home directory
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}
