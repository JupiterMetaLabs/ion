package ion

import (
	"io"

	"gopkg.in/natefinch/lumberjack.v2"
)

// NewFileWriter creates a file writer with rotation using lumberjack.
// Returns nil if the path is empty.
func NewFileWriter(cfg FileConfig) io.Writer {
	if cfg.Path == "" {
		return nil
	}

	maxSize := cfg.MaxSizeMB
	if maxSize <= 0 {
		maxSize = 100 // Default 100MB
	}

	maxAge := cfg.MaxAgeDays
	if maxAge <= 0 {
		maxAge = 7 // Default 7 days
	}

	maxBackups := cfg.MaxBackups
	if maxBackups <= 0 {
		maxBackups = 5 // Default 5 backups
	}

	return &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    maxSize,    // megabytes
		MaxAge:     maxAge,     // days
		MaxBackups: maxBackups, // number of backups
		Compress:   cfg.Compress,
		LocalTime:  true,
	}
}
