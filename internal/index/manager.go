package index

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/atregu/ipfs-publisher/internal/logger"
)

// Record represents a single entry in the NDJSON index
type Record struct {
	ID        int    `json:"id"`
	CID       string `json:"CID"`
	Filename  string `json:"filename"`
	Extension string `json:"extension"`
}

// Manager handles NDJSON index operations
type Manager struct {
	indexPath string
	records   map[string]*Record
	nextID    int
}

// New creates a new index manager
func New(indexPath string) *Manager {
	return &Manager{
		indexPath: expandPath(indexPath),
		records:   make(map[string]*Record),
		nextID:    1,
	}
}

// Load loads the index from disk
func (m *Manager) Load() error {
	log := logger.Get()

	dir := filepath.Dir(m.indexPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	if _, err := os.Stat(m.indexPath); os.IsNotExist(err) {
		log.Info("Index file does not exist, will create new one")
		return nil
	}

	file, err := os.Open(m.indexPath)
	if err != nil {
		return fmt.Errorf("failed to open index file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if line == "" {
			continue
		}

		var record Record
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			log.Warnf("Failed to parse line %d: %v", lineNum, err)
			continue
		}

		m.records[record.Filename] = &record

		if record.ID >= m.nextID {
			m.nextID = record.ID + 1
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading index file: %w", err)
	}

	log.Infof("Loaded %d records from index (next ID: %d)", len(m.records), m.nextID)
	return nil
}

// Save writes the index to disk
func (m *Manager) Save() error {
	log := logger.Get()

	tmpPath := m.indexPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create temp index file: %w", err)
	}

	writer := bufio.NewWriter(file)

	recordCount := 0
	for _, record := range m.records {
		data, err := json.Marshal(record)
		if err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to marshal record: %w", err)
		}

		if _, err := writer.Write(data); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write record: %w", err)
		}

		if _, err := writer.WriteString("\n"); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("failed to write newline: %w", err)
		}

		recordCount++
	}

	if err := writer.Flush(); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to flush writer: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close file: %w", err)
	}

	if err := os.Rename(tmpPath, m.indexPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	log.Infof("Saved %d records to index", recordCount)
	return nil
}

// Add adds a new file to the index
func (m *Manager) Add(filename, cid, extension string) *Record {
	record := &Record{
		ID:        m.nextID,
		CID:       cid,
		Filename:  filename,
		Extension: extension,
	}

	m.records[filename] = record
	m.nextID++

	return record
}

// Update updates the CID for an existing file
func (m *Manager) Update(filename, cid string) (*Record, error) {
	record, exists := m.records[filename]
	if !exists {
		return nil, fmt.Errorf("record not found: %s", filename)
	}

	record.CID = cid
	return record, nil
}

// Delete removes a record by filename
func (m *Manager) Delete(filename string) error {
	if _, exists := m.records[filename]; !exists {
		return fmt.Errorf("record not found: %s", filename)
	}

	delete(m.records, filename)
	return nil
}

// Get retrieves a record by filename
func (m *Manager) Get(filename string) (*Record, bool) {
	record, exists := m.records[filename]
	return record, exists
}

// Count returns the number of records
func (m *Manager) Count() int {
	return len(m.records)
}

// GetPath returns the index file path
func (m *Manager) GetPath() string {
	return m.indexPath
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
