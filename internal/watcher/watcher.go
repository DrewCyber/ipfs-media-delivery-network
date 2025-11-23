package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/atregu/ipfs-publisher/internal/logger"
	"github.com/fsnotify/fsnotify"
)

// FileEvent represents a file system event
type FileEvent struct {
	Path      string
	EventType EventType
	Timestamp time.Time
}

// EventType represents the type of file system event
type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
	EventRename
)

func (e EventType) String() string {
	switch e {
	case EventCreate:
		return "CREATE"
	case EventModify:
		return "MODIFY"
	case EventDelete:
		return "DELETE"
	case EventRename:
		return "RENAME"
	default:
		return "UNKNOWN"
	}
}

// Watcher monitors directories for file changes
type Watcher struct {
	watcher    *fsnotify.Watcher
	extensions map[string]bool
	debouncer  *debouncer
	eventChan  chan FileEvent
	mu         sync.RWMutex
	started    bool
}

// Config holds watcher configuration
type Config struct {
	Directories    []string
	Extensions     []string
	DebounceDelay  time.Duration
	EventQueueSize int
}

// NewWatcher creates a new file watcher
func NewWatcher(cfg *Config) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	// Build extension map
	extMap := make(map[string]bool)
	for _, ext := range cfg.Extensions {
		extMap[strings.ToLower(ext)] = true
	}

	debounceDelay := cfg.DebounceDelay
	if debounceDelay == 0 {
		debounceDelay = 300 * time.Millisecond // Default debounce delay
	}

	eventQueueSize := cfg.EventQueueSize
	if eventQueueSize == 0 {
		eventQueueSize = 100
	}

	w := &Watcher{
		watcher:    fsWatcher,
		extensions: extMap,
		debouncer:  newDebouncer(debounceDelay),
		eventChan:  make(chan FileEvent, eventQueueSize),
	}

	return w, nil
}

// Start starts watching directories
func (w *Watcher) Start(directories []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return fmt.Errorf("watcher already started")
	}

	log := logger.Get()

	// Add directories to watch
	for _, dir := range directories {
		// Expand ~ in path
		expandedDir := expandPath(dir)

		// Walk directory tree and add all subdirectories
		err := filepath.Walk(expandedDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				log.Warnf("Failed to access path %s: %v", path, err)
				return nil // Continue walking
			}

			if info.IsDir() {
				// Skip hidden directories
				if strings.HasPrefix(info.Name(), ".") && path != expandedDir {
					return filepath.SkipDir
				}

				if err := w.watcher.Add(path); err != nil {
					log.Warnf("Failed to watch directory %s: %v", path, err)
					return nil
				}
				log.Debugf("Watching directory: %s", path)
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to walk directory %s: %w", expandedDir, err)
		}

		log.Infof("Started watching: %s", expandedDir)
	}

	// Start event processing
	go w.processEvents()

	w.started = true
	return nil
}

// processEvents processes file system events
func (w *Watcher) processEvents() {
	log := logger.Get()
	log.Debug("File watcher event processor started")

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			log.Errorf("Watcher error: %v", err)
		}
	}
}

// handleEvent processes a single fsnotify event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	log := logger.Get()

	// Get file info
	info, err := os.Stat(event.Name)

	// Check if file should be ignored
	if err == nil && info.IsDir() {
		// New directory created - add it to watch list
		if event.Op&fsnotify.Create == fsnotify.Create {
			if !strings.HasPrefix(filepath.Base(event.Name), ".") {
				if err := w.watcher.Add(event.Name); err != nil {
					log.Warnf("Failed to watch new directory %s: %v", event.Name, err)
				} else {
					log.Debugf("Started watching new directory: %s", event.Name)
				}
			}
		}
		return // Ignore directory events
	}

	// Ignore hidden files and temporary files
	basename := filepath.Base(event.Name)
	if strings.HasPrefix(basename, ".") || strings.HasSuffix(basename, "~") {
		return
	}

	// Check extension
	if !w.hasValidExtension(event.Name) {
		return
	}

	// Determine event type
	var eventType EventType

	if event.Op&fsnotify.Create == fsnotify.Create {
		eventType = EventCreate
	} else if event.Op&fsnotify.Write == fsnotify.Write {
		eventType = EventModify
	} else if event.Op&fsnotify.Remove == fsnotify.Remove {
		eventType = EventDelete
	} else if event.Op&fsnotify.Rename == fsnotify.Rename {
		// Rename shows up as RENAME (old file) and CREATE (new file)
		// We treat RENAME without CREATE as delete
		eventType = EventDelete
	} else {
		// Ignore other events
		return
	}

	log.Debugf("File event: %s %s", eventType, event.Name)

	// Debounce the event
	w.debouncer.debounce(event.Name, func() {
		w.eventChan <- FileEvent{
			Path:      event.Name,
			EventType: eventType,
			Timestamp: time.Now(),
		}
	})
}

// hasValidExtension checks if file has valid extension
func (w *Watcher) hasValidExtension(path string) bool {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	ext = strings.ToLower(ext)
	return w.extensions[ext]
}

// Events returns the channel for receiving file events
func (w *Watcher) Events() <-chan FileEvent {
	return w.eventChan
}

// Stop stops the watcher
func (w *Watcher) Stop() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.started {
		return nil
	}

	log := logger.Get()
	log.Info("Stopping file watcher...")

	if err := w.watcher.Close(); err != nil {
		return fmt.Errorf("failed to close watcher: %w", err)
	}

	close(w.eventChan)
	w.debouncer.stop()

	w.started = false
	log.Info("File watcher stopped")
	return nil
}

// debouncer handles debouncing of events
type debouncer struct {
	delay  time.Duration
	timers map[string]*time.Timer
	mu     sync.Mutex
}

func newDebouncer(delay time.Duration) *debouncer {
	return &debouncer{
		delay:  delay,
		timers: make(map[string]*time.Timer),
	}
}

func (d *debouncer) debounce(key string, fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Cancel existing timer for this key
	if timer, exists := d.timers[key]; exists {
		timer.Stop()
	}

	// Create new timer
	d.timers[key] = time.AfterFunc(d.delay, func() {
		fn()
		d.mu.Lock()
		delete(d.timers, key)
		d.mu.Unlock()
	})
}

func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, timer := range d.timers {
		timer.Stop()
	}
	d.timers = make(map[string]*time.Timer)
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
