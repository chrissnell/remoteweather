// Package log provides centralized logging functionality using zap logger.
package log

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.SugaredLogger
var baseLogger *zap.Logger
var logBuffer *LogBuffer

// LogBuffer is a thread-safe circular buffer for capturing log entries
type LogBuffer struct {
	mutex       sync.RWMutex
	entries     []LogEntry
	maxSize     int
	index       int
	subscribers []chan LogEntry // Channels for WebSocket notifications
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Caller    string                 `json:"caller,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// NewLogBuffer creates a new log buffer with the specified maximum size
func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, maxSize),
		maxSize: maxSize,
	}
}

// Write implements zapcore.WriteSyncer interface
func (lb *LogBuffer) Write(data []byte) (int, error) {
	var logData map[string]interface{}
	if err := json.Unmarshal(data, &logData); err != nil {
		// If we can't parse as JSON, treat as plain text
		lb.AddEntry(LogEntry{
			Timestamp: time.Now(),
			Level:     "unknown",
			Message:   string(data),
		})
		return len(data), nil
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Fields:    make(map[string]interface{}),
	}

	// Parse timestamp - zap can use different field names and formats
	if ts, ok := logData["ts"]; ok {
		if parsed := parseTimestamp(ts); !parsed.IsZero() {
			entry.Timestamp = parsed
		}
	} else if ts, ok := logData["time"]; ok {
		if parsed := parseTimestamp(ts); !parsed.IsZero() {
			entry.Timestamp = parsed
		}
	} else if ts, ok := logData["timestamp"]; ok {
		if parsed := parseTimestamp(ts); !parsed.IsZero() {
			entry.Timestamp = parsed
		}
	} else if ts, ok := logData["@timestamp"]; ok {
		if parsed := parseTimestamp(ts); !parsed.IsZero() {
			entry.Timestamp = parsed
		}
	}

	if level, ok := logData["level"]; ok {
		entry.Level = fmt.Sprintf("%v", level)
	}

	if msg, ok := logData["msg"]; ok {
		entry.Message = fmt.Sprintf("%v", msg)
	} else if msg, ok := logData["message"]; ok {
		entry.Message = fmt.Sprintf("%v", msg)
	}

	if caller, ok := logData["caller"]; ok {
		entry.Caller = fmt.Sprintf("%v", caller)
	}

	// Add any additional fields
	excludeFields := map[string]bool{
		"ts": true, "time": true, "timestamp": true, "@timestamp": true,
		"level": true, "msg": true, "message": true, "caller": true,
	}
	for k, v := range logData {
		if !excludeFields[k] {
			entry.Fields[k] = v
		}
	}

	lb.AddEntry(entry)
	return len(data), nil
}

// parseTimestamp attempts to parse various timestamp formats
func parseTimestamp(ts interface{}) time.Time {
	switch v := ts.(type) {
	case float64:
		// Unix timestamp in seconds or nanoseconds
		if v > 1e10 {
			// Likely nanoseconds
			return time.Unix(0, int64(v))
		} else {
			// Likely seconds
			return time.Unix(int64(v), 0)
		}
	case int64:
		// Unix timestamp
		if v > 1e10 {
			// Likely nanoseconds
			return time.Unix(0, v)
		} else {
			// Likely seconds
			return time.Unix(v, 0)
		}
	case string:
		// Try to parse as RFC3339 or other common formats
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05.000Z07:00",
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02 15:04:05",
		}
		for _, format := range formats {
			if parsed, err := time.Parse(format, v); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

// Sync implements zapcore.WriteSyncer interface
func (lb *LogBuffer) Sync() error {
	return nil
}

// addEntry adds a log entry to the circular buffer
// AddEntry adds a log entry to the buffer
func (lb *LogBuffer) AddEntry(entry LogEntry) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	lb.entries[lb.index] = entry
	lb.index = (lb.index + 1) % lb.maxSize

	// Notify subscribers
	for _, sub := range lb.subscribers {
		select {
		case sub <- entry:
		default:
			// If subscriber channel is full, drop the message
			// This is a simple way to handle backpressure,
			// but a more robust solution might involve a queue.
		}
	}
}

// GetLogs returns all current log entries and optionally clears the buffer
func (lb *LogBuffer) GetLogs(clear bool) []LogEntry {
	if clear {
		lb.mutex.Lock()
		defer lb.mutex.Unlock()
	} else {
		lb.mutex.RLock()
		defer lb.mutex.RUnlock()
	}

	var result []LogEntry

	// Collect entries in chronological order
	for i := 0; i < lb.maxSize; i++ {
		idx := (lb.index + i) % lb.maxSize
		if !lb.entries[idx].Timestamp.IsZero() {
			result = append(result, lb.entries[idx])
		}
	}

	if clear {
		// Clear the buffer
		lb.entries = make([]LogEntry, lb.maxSize)
		lb.index = 0
	}

	return result
}

// Subscribe adds a channel to receive new log entries as they arrive
func (lb *LogBuffer) Subscribe() chan LogEntry {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	ch := make(chan LogEntry, 10) // Buffer 10 entries to handle bursts
	lb.subscribers = append(lb.subscribers, ch)
	return ch
}

// Unsubscribe removes a channel from receiving log entries
func (lb *LogBuffer) Unsubscribe(ch chan LogEntry) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	for i, sub := range lb.subscribers {
		if sub == ch {
			// Remove subscriber from slice
			lb.subscribers = append(lb.subscribers[:i], lb.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// multiWriter combines multiple writers
type multiWriter struct {
	writers []zapcore.WriteSyncer
}

func (mw *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		if _, err := w.Write(p); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (mw *multiWriter) Sync() error {
	for _, w := range mw.writers {
		if err := w.Sync(); err != nil {
			return err
		}
	}
	return nil
}

// Init initializes the package-level logger with buffering
func Init(debug bool) error {
	// Create log buffer (500 entries max)
	logBuffer = NewLogBuffer(500)

	// Create encoder config
	var encoderConfig zapcore.EncoderConfig
	if debug {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
	}

	// Configure the JSON encoder with consistent field names for better parsing
	jsonEncoderConfig := encoderConfig
	jsonEncoderConfig.TimeKey = "timestamp"
	jsonEncoderConfig.LevelKey = "level"
	jsonEncoderConfig.MessageKey = "message"
	jsonEncoderConfig.CallerKey = "caller"
	jsonEncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	jsonEncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	// Create JSON encoder for both console and buffer
	jsonEncoder := zapcore.NewJSONEncoder(jsonEncoderConfig)

	// Create cores
	var level zapcore.Level
	if debug {
		level = zapcore.DebugLevel
	} else {
		level = zapcore.InfoLevel
	}

	// Console core (stdout) - now using JSON format
	consoleCore := zapcore.NewCore(
		jsonEncoder,
		zapcore.AddSync(os.Stdout),
		level,
	)

	// Buffer core (in-memory)
	bufferCore := zapcore.NewCore(
		jsonEncoder,
		zapcore.AddSync(logBuffer),
		level,
	)

	// Combine cores
	core := zapcore.NewTee(consoleCore, bufferCore)

	// Create logger
	baseLogger = zap.New(core, zap.AddCaller())
	log = baseLogger.Sugar()

	return nil
}

// GetLogBuffer returns the log buffer instance
func GetLogBuffer() *LogBuffer {
	return logBuffer
}

// GetZapLogger returns the base zap logger for cases where it's needed (like GORM)
func GetZapLogger() *zap.Logger {
	if baseLogger == nil {
		// Fallback logger if not initialized
		baseLogger, _ = zap.NewProduction()
		log = baseLogger.Sugar()
	}
	return baseLogger
}

// GetSugaredLogger returns the sugared logger instance
func GetSugaredLogger() *zap.SugaredLogger {
	if log == nil {
		// Fallback logger if not initialized
		baseLogger, _ = zap.NewProduction()
		log = baseLogger.Sugar()
	}
	return log
}

// Sync flushes any buffered log entries
func Sync() {
	if log != nil {
		log.Sync()
	}
}

// Package-level convenience functions
func Debug(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Debug(args...)
}

func Debugf(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Debugf(template, args...)
}

func Debugw(msg string, keysAndValues ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Debugw(msg, keysAndValues...)
}

func Info(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Info(args...)
}

func Infof(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Infof(template, args...)
}

func Infow(msg string, keysAndValues ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Infow(msg, keysAndValues...)
}

func Warn(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Warn(args...)
}

func Warnf(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Warnf(template, args...)
}

func Warnw(msg string, keysAndValues ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Warnw(msg, keysAndValues...)
}

func Error(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Error(args...)
}

func Errorf(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Errorf(template, args...)
}

func Errorw(msg string, keysAndValues ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Errorw(msg, keysAndValues...)
}

func Errorln(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Error(args...)
}

func Fatal(args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Fatal(args...)
	os.Exit(1)
}

func Fatalf(template string, args ...interface{}) {
	baseLogger.WithOptions(zap.AddCallerSkip(1)).Sugar().Fatalf(template, args...)
	os.Exit(1)
}
