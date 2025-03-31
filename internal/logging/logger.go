package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/joho/godotenv"
)

// LogLevel represents the verbosity level of logging
type LogLevel int

const (
	// LogLevelError only logs error messages
	LogLevelError LogLevel = iota
	// LogLevelInfo logs info and error messages
	LogLevelInfo
	// LogLevelDebug logs debug, info, and error messages
	LogLevelDebug

	// Default log level if not specified
	defaultLogLevel = LogLevelInfo
)

var (
	// Default log directory
	defaultLogDir = "logs"

	// Global logger instance
	instance *Logger
	once     sync.Once

	// Environment file path
	envFile = ".env"

	// Environment file path mutex
	envMutex sync.Mutex

	logDir = "logs" // Default log directory
)

// Logger wraps the standard logger with additional functionality
type Logger struct {
	level LogLevel
	file  *os.File
}

// SetEnvFile sets the path to the environment file
func SetEnvFile(path string) {
	envMutex.Lock()
	defer envMutex.Unlock()

	envFile = path
	instance = nil // Reset logger instance to force reinitialization
	once = sync.Once{}

	// Load environment variables from the new file
	if err := godotenv.Load(path); err != nil {
		log.Printf("Warning: Error loading %s file: %v", path, err)
	}
}

// GetLogger returns the singleton logger instance
func GetLogger() *Logger {
	once.Do(func() {
		// Create logs directory if it doesn't exist
		if err := os.MkdirAll(defaultLogDir, 0755); err != nil {
			log.Printf("Error creating log directory: %v", err)
			return
		}

		// Open log file
		logFile := filepath.Join(defaultLogDir, "solana-monitor.log")
		file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Error opening log file: %v", err)
			return
		}

		// Get log level from environment
		level := getLogLevelFromEnv()

		// Create logger instance
		instance = &Logger{
			level: level,
			file:  file,
		}

		log.Printf("[INFO] Logger initialized with level: %s", FormatLogLevel(level))
	})
	return instance
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LogLevelError, format, args...)
}

// Info logs an info message if log level is Info or Debug
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LogLevelInfo {
		l.log(LogLevelInfo, format, args...)
	}
}

// Debug logs a debug message if log level is Debug
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LogLevelDebug {
		l.log(LogLevelDebug, format, args...)
	}
}

// SetLogLevel sets the logging level
func (l *Logger) SetLogLevel(level LogLevel) {
	l.level = level
	l.Info("Log level changed to: %s", FormatLogLevel(level))
}

// GetLogLevel returns the current logging level
func (l *Logger) GetLogLevel() LogLevel {
	return l.level
}

// FormatLogLevel converts a LogLevel to its string representation
func FormatLogLevel(level LogLevel) string {
	switch level {
	case LogLevelError:
		return "ERROR"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", level)
	}
}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	logMessage := fmt.Sprintf("[%s] %s\n", FormatLogLevel(level), message)

	// Write to file
	if l.file != nil {
		if _, err := l.file.WriteString(logMessage); err != nil {
			log.Printf("Error writing to log file: %v", err)
		}
	}

	// Write to stdout
	log.Print(logMessage)
}

func getLogLevelFromEnv() LogLevel {
	envMutex.Lock()
	defer envMutex.Unlock()

	// Try to load environment variables from file first
	if err := godotenv.Load(envFile); err == nil {
		level := os.Getenv("LOG_LEVEL")
		switch level {
		case "ERROR":
			return LogLevelError
		case "DEBUG":
			return LogLevelDebug
		case "INFO":
			return LogLevelInfo
		}
	}

	// If file loading fails or level is not set, check environment variable directly
	level := os.Getenv("LOG_LEVEL")
	switch level {
	case "ERROR":
		return LogLevelError
	case "DEBUG":
		return LogLevelDebug
	case "INFO", "":
		return LogLevelInfo
	default:
		return LogLevelInfo
	}
}

// SetLogDirectory sets the directory where all logs will be stored
func SetLogDirectory(dir string) {
	logDir = dir
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create log directory: %v", err)
	}
}

// CreateLogFile creates a log file in the configured log directory
func CreateLogFile(name string) (*os.File, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// Create log file path
	logPath := filepath.Join(logDir, name)

	// Open log file
	file, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file %s: %v", logPath, err)
	}

	return file, nil
}

// CreateMultiWriter creates a writer that writes to both file and stdout
func CreateMultiWriter(file *os.File) io.Writer {
	return io.MultiWriter(os.Stdout, file)
}
