package utils

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	// MaxFilenameLength is the maximum allowed filename length
	MaxFilenameLength = 255
)

// SanitizeFilename sanitizes a filename by removing or replacing unsafe characters
// while preserving the file extension
func SanitizeFilename(filename string) string {
	if filename == "" {
		return filename
	}

	// Split name and extension
	ext := filepath.Ext(filename)
	name := strings.TrimSuffix(filename, ext)

	// Remove leading/trailing dots and spaces
	name = strings.TrimSpace(name)
	name = strings.Trim(name, ".")

	// Replace problematic characters
	name = strings.Map(func(r rune) rune {
		// Allow alphanumeric, spaces, and common safe chars
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == ' ' || r == '-' || r == '_' {
			return r
		}
		// Replace other characters with underscore
		return '_'
	}, name)

	// Collapse multiple underscores
	for strings.Contains(name, "__") {
		name = strings.ReplaceAll(name, "__", "_")
	}

	// Remove leading/trailing underscores
	name = strings.Trim(name, "_")

	// Combine name and extension
	sanitized := name + ext

	// Truncate if too long
	if len(sanitized) > MaxFilenameLength {
		// Keep the extension, truncate the name
		maxNameLen := MaxFilenameLength - len(ext)
		if maxNameLen > 0 {
			name = name[:maxNameLen]
		}
		sanitized = name + ext
	}

	return sanitized
}

// IsValidPath checks if a path is safe and doesn't contain path traversal attempts
func IsValidPath(path string) bool {
	// Check for empty path
	if path == "" {
		return false
	}

	// Clean the path
	cleaned := filepath.Clean(path)

	// Check for path traversal attempts
	if strings.Contains(cleaned, "..") {
		return false
	}

	// Check for absolute path requirement (optional)
	// In our case, we want absolute paths
	if !filepath.IsAbs(cleaned) {
		return false
	}

	return true
}

// IsHiddenFile checks if a file is hidden (starts with .)
func IsHiddenFile(name string) bool {
	return strings.HasPrefix(filepath.Base(name), ".")
}

// IsTempFile checks if a file is a temporary file (ends with ~)
func IsTempFile(name string) bool {
	return strings.HasSuffix(filepath.Base(name), "~")
}

// ShouldIgnoreFile checks if a file should be ignored based on common patterns
func ShouldIgnoreFile(name string) bool {
	base := filepath.Base(name)

	// Hidden files
	if IsHiddenFile(base) {
		return true
	}

	// Temporary files
	if IsTempFile(base) {
		return true
	}

	// Common temp file patterns
	tempPatterns := []string{
		".tmp", ".temp", ".swp", ".swo", ".swn",
		".DS_Store", "Thumbs.db", "desktop.ini",
	}

	nameLower := strings.ToLower(base)
	for _, pattern := range tempPatterns {
		if strings.HasSuffix(nameLower, pattern) || nameLower == strings.ToLower(pattern) {
			return true
		}
	}

	return false
}

// HasExtension checks if a file has one of the allowed extensions (case-insensitive)
func HasExtension(filename string, allowedExts []string) bool {
	if len(allowedExts) == 0 {
		return true // If no extensions specified, allow all
	}

	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	extLower := strings.ToLower(ext)

	for _, allowed := range allowedExts {
		allowedLower := strings.ToLower(strings.TrimPrefix(allowed, "."))
		if extLower == allowedLower {
			return true
		}
	}

	return false
}

// FormatBytes formats a byte count as a human-readable string
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
