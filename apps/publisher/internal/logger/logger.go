package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

var log *logrus.Logger

// Init initializes the logger with the specified configuration
func Init(level, logFile string, maxSize, maxBackups int, console bool) error {
	log = logrus.New()

	// Set log level
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}
	log.SetLevel(lvl)

	// Set formatter
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// Expand tilde in log file path
	if strings.HasPrefix(logFile, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		logFile = filepath.Join(home, logFile[1:])
	}

	// Create log directory if it doesn't exist
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Set up file output with rotation
	fileWriter := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    maxSize, // megabytes
		MaxBackups: maxBackups,
		MaxAge:     0,    // days (0 = no age limit)
		Compress:   true, // compress old log files
	}

	// Set output based on console flag
	if console {
		log.SetOutput(io.MultiWriter(os.Stdout, fileWriter))
	} else {
		log.SetOutput(fileWriter)
	}

	return nil
}

// Get returns the logger instance
func Get() *logrus.Logger {
	if log == nil {
		// Return a default logger if not initialized
		log = logrus.New()
		log.SetLevel(logrus.InfoLevel)
		log.SetOutput(os.Stdout)
	}
	return log
}

// Debug logs a debug message
func Debug(args ...interface{}) {
	Get().Debug(args...)
}

// Debugf logs a formatted debug message
func Debugf(format string, args ...interface{}) {
	Get().Debugf(format, args...)
}

// Info logs an info message
func Info(args ...interface{}) {
	Get().Info(args...)
}

// Infof logs a formatted info message
func Infof(format string, args ...interface{}) {
	Get().Infof(format, args...)
}

// Warn logs a warning message
func Warn(args ...interface{}) {
	Get().Warn(args...)
}

// Warnf logs a formatted warning message
func Warnf(format string, args ...interface{}) {
	Get().Warnf(format, args...)
}

// Error logs an error message
func Error(args ...interface{}) {
	Get().Error(args...)
}

// Errorf logs a formatted error message
func Errorf(format string, args ...interface{}) {
	Get().Errorf(format, args...)
}

// Fatal logs a fatal message and exits
func Fatal(args ...interface{}) {
	Get().Fatal(args...)
}

// Fatalf logs a formatted fatal message and exits
func Fatalf(format string, args ...interface{}) {
	Get().Fatalf(format, args...)
}
