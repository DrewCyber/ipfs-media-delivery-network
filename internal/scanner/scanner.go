package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/atregu/ipfs-publisher/internal/logger"
	"github.com/atregu/ipfs-publisher/internal/utils"
)

// FileInfo represents information about a scanned file
type FileInfo struct {
	Path      string
	Name      string
	Extension string
	Size      int64
	ModTime   int64
}

// Scanner scans directories for media files
type Scanner struct {
	directories []string
	extensions  map[string]bool
}

// New creates a new Scanner
func New(directories []string, extensions []string) *Scanner {
	extMap := make(map[string]bool)
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	return &Scanner{
		directories: directories,
		extensions:  extMap,
	}
}

// Scan recursively scans all configured directories
func (s *Scanner) Scan() ([]FileInfo, error) {
	log := logger.Get()
	var files []FileInfo

	for _, dir := range s.directories {
		expandedDir := expandPath(dir)
		log.Infof("Scanning directory: %s", expandedDir)

		info, err := os.Stat(expandedDir)
		if err != nil {
			if os.IsNotExist(err) {
				log.Warnf("Directory does not exist: %s", expandedDir)
				continue
			}
			return nil, fmt.Errorf("failed to stat directory %s: %w", expandedDir, err)
		}

		if !info.IsDir() {
			log.Warnf("Path is not a directory: %s", expandedDir)
			continue
		}

		err = filepath.Walk(expandedDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// Handle permission errors gracefully
				if os.IsPermission(err) {
					log.Warnf("Permission denied: %s (skipping)", path)
					return nil
				}
				log.Warnf("Error accessing path %s: %v", path, err)
				return nil
			}

			if info.IsDir() {
				return nil
			}

			// Check for symlinks
			if info.Mode()&os.ModeSymlink != 0 {
				log.Debugf("Skipping symbolic link: %s", path)
				return nil
			}

			// Use utility function to check if file should be ignored
			if utils.ShouldIgnoreFile(info.Name()) {
				log.Debugf("Skipping ignored file: %s", path)
				return nil
			}

			ext := filepath.Ext(info.Name())
			if ext == "" {
				log.Debugf("Skipping file without extension: %s", path)
				return nil
			}

			ext = strings.ToLower(strings.TrimPrefix(ext, "."))

			if !s.extensions[ext] {
				log.Debugf("Skipping file with non-matching extension: %s", path)
				return nil
			}

			// Check filename length
			if len(info.Name()) > utils.MaxFilenameLength {
				log.Warnf("Filename too long (%d chars), skipping: %s", len(info.Name()), path)
				return nil
			}

			files = append(files, FileInfo{
				Path:      path,
				Name:      info.Name(),
				Extension: ext,
				Size:      info.Size(),
				ModTime:   info.ModTime().Unix(),
			})

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to walk directory %s: %w", expandedDir, err)
		}
	}

	log.Infof("Found %d files matching criteria", len(files))
	return files, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}
