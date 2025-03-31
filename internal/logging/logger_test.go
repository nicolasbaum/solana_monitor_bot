package logging

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func resetLogger() {
	instance = nil
	once = sync.Once{}
}

func setupTest(t *testing.T) (string, func()) {
	// Create a temporary log directory for testing
	tmpDir := "test_logs"
	defaultLogDir = tmpDir

	// Create a temporary .env file
	envFile := ".env.test"
	err := os.WriteFile(envFile, []byte(""), 0644)
	assert.NoError(t, err)
	SetEnvFile(envFile)

	// Cleanup function
	cleanup := func() {
		os.RemoveAll(tmpDir)
		os.Remove(envFile)
		resetLogger()
		os.Setenv("LOG_LEVEL", "") // Reset environment variable
		SetEnvFile(".env")         // Reset env file path
	}

	return tmpDir, cleanup
}

func TestGetLogger(t *testing.T) {
	tmpDir, cleanup := setupTest(t)
	defer cleanup()

	tests := []struct {
		name          string
		envContent    string
		expectedLevel LogLevel
	}{
		{
			name:          "Default log level (no env var)",
			envContent:    "",
			expectedLevel: LogLevelInfo,
		},
		{
			name:          "Debug level from .env",
			envContent:    "LOG_LEVEL=DEBUG",
			expectedLevel: LogLevelDebug,
		},
		{
			name:          "Error level from .env",
			envContent:    "LOG_LEVEL=ERROR",
			expectedLevel: LogLevelError,
		},
		{
			name:          "Info level from .env",
			envContent:    "LOG_LEVEL=INFO",
			expectedLevel: LogLevelInfo,
		},
		{
			name:          "Invalid level defaults to Info",
			envContent:    "LOG_LEVEL=INVALID",
			expectedLevel: LogLevelInfo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup()

			// Set environment variable directly
			if tt.envContent != "" {
				parts := strings.Split(tt.envContent, "=")
				if len(parts) == 2 {
					os.Setenv(parts[0], parts[1])
				}
			}

			// Create .env file with test content
			err := os.WriteFile(".env.test", []byte(tt.envContent), 0644)
			assert.NoError(t, err)

			logger := GetLogger()
			assert.NotNil(t, logger)
			assert.Equal(t, tt.expectedLevel, logger.GetLogLevel())

			// Verify log file was created
			_, err = os.Stat(filepath.Join(tmpDir, "solana-monitor.log"))
			assert.NoError(t, err)
		})
	}
}

func TestLogLevels(t *testing.T) {
	tmpDir, cleanup := setupTest(t)
	defer cleanup()

	tests := []struct {
		name     string
		level    LogLevel
		logFunc  func(logger *Logger)
		message  string
		expected bool // whether message should appear in log
	}{
		{
			name:  "Error message with ERROR level",
			level: LogLevelError,
			logFunc: func(logger *Logger) {
				logger.Error("test error message")
			},
			message:  "test error message",
			expected: true,
		},
		{
			name:  "Info message with ERROR level",
			level: LogLevelError,
			logFunc: func(logger *Logger) {
				logger.Info("test info message")
			},
			message:  "test info message",
			expected: false,
		},
		{
			name:  "Debug message with ERROR level",
			level: LogLevelError,
			logFunc: func(logger *Logger) {
				logger.Debug("test debug message")
			},
			message:  "test debug message",
			expected: false,
		},
		{
			name:  "Error message with INFO level",
			level: LogLevelInfo,
			logFunc: func(logger *Logger) {
				logger.Error("test error message")
			},
			message:  "test error message",
			expected: true,
		},
		{
			name:  "Info message with INFO level",
			level: LogLevelInfo,
			logFunc: func(logger *Logger) {
				logger.Info("test info message")
			},
			message:  "test info message",
			expected: true,
		},
		{
			name:  "Debug message with INFO level",
			level: LogLevelInfo,
			logFunc: func(logger *Logger) {
				logger.Debug("test debug message")
			},
			message:  "test debug message",
			expected: false,
		},
		{
			name:  "Error message with DEBUG level",
			level: LogLevelDebug,
			logFunc: func(logger *Logger) {
				logger.Error("test error message")
			},
			message:  "test error message",
			expected: true,
		},
		{
			name:  "Info message with DEBUG level",
			level: LogLevelDebug,
			logFunc: func(logger *Logger) {
				logger.Info("test info message")
			},
			message:  "test info message",
			expected: true,
		},
		{
			name:  "Debug message with DEBUG level",
			level: LogLevelDebug,
			logFunc: func(logger *Logger) {
				logger.Debug("test debug message")
			},
			message:  "test debug message",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset logger instance for each test
			resetLogger()

			// Create new logger with test level
			logger := GetLogger()
			logger.SetLogLevel(tt.level)

			// Execute log function
			tt.logFunc(logger)

			// Read log file
			logFile := filepath.Join(tmpDir, "solana-monitor.log")
			content, err := os.ReadFile(logFile)
			assert.NoError(t, err)

			// Check if message appears in log as expected
			containsMessage := strings.Contains(string(content), tt.message)
			assert.Equal(t, tt.expected, containsMessage, "Log message presence mismatch")
		})
	}
}

func TestFormatLogLevel(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelError, "ERROR"},
		{LogLevelInfo, "INFO"},
		{LogLevelDebug, "DEBUG"},
		{LogLevel(99), "UNKNOWN(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatLogLevel(tt.level))
		})
	}
}

func TestSetEnvFile(t *testing.T) {
	// Create temporary files
	tmpDir := "test_logs"
	defaultLogDir = tmpDir
	defer os.RemoveAll(tmpDir)

	env1 := ".env.test1"
	env2 := ".env.test2"
	defer os.Remove(env1)
	defer os.Remove(env2)

	// Create first .env file with DEBUG level
	err := os.WriteFile(env1, []byte("LOG_LEVEL=DEBUG"), 0644)
	assert.NoError(t, err)

	// Create second .env file with ERROR level
	err = os.WriteFile(env2, []byte("LOG_LEVEL=ERROR"), 0644)
	assert.NoError(t, err)

	// Reset logger before each test
	resetLogger()

	// Test first .env file
	SetEnvFile(env1)
	os.Setenv("LOG_LEVEL", "DEBUG")
	logger := GetLogger()
	assert.Equal(t, LogLevelDebug, logger.GetLogLevel())

	// Reset logger before testing second file
	resetLogger()

	// Test second .env file
	SetEnvFile(env2)
	os.Setenv("LOG_LEVEL", "ERROR")
	logger = GetLogger()
	assert.Equal(t, LogLevelError, logger.GetLogLevel())
}
