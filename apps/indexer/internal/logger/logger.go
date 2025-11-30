package logger

import (
	"fmt"
	"io"
	"os"

	"github.com/sirupsen/logrus"
)

var log *logrus.Logger

// Init initializes the logger with the specified configuration
func Init(level, format, output, filePath string) error {
	log = logrus.New()

	// Set log level
	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		return err
	}
	log.SetLevel(lvl)

	// Set log format
	if format == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp: true,
		})
	}

	// Set output
	switch output {
	case "file":
		if filePath == "" {
			return fmt.Errorf("file path is required when output is 'file'")
		}
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		log.SetOutput(file)
	case "both":
		if filePath == "" {
			return fmt.Errorf("file path is required when output is 'both'")
		}
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		log.SetOutput(io.MultiWriter(os.Stdout, file))
	default:
		log.SetOutput(os.Stdout)
	}

	return nil
}

// Get returns the logger instance
func Get() *logrus.Logger {
	if log == nil {
		log = logrus.New()
	}
	return log
}
