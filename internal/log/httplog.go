package log

import (
	"fmt"
	"sync"
	"time"
)

// HTTP log buffer is separate from the main log buffer
var httpLogBuffer *LogBuffer
var httpLogBufferOnce sync.Once

// HTTPLogEntry represents an HTTP request/response log entry
type HTTPLogEntry struct {
	Timestamp    time.Time              `json:"timestamp"`
	Method       string                 `json:"method"`
	Path         string                 `json:"path"`
	Status       int                    `json:"status"`
	Duration     time.Duration          `json:"duration"`
	Size         int                    `json:"size"`
	RemoteAddr   string                 `json:"remote_addr"`
	UserAgent    string                 `json:"user_agent"`
	Website      string                 `json:"website,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Fields       map[string]any `json:"fields,omitempty"`
}

// GetHTTPLogBuffer returns the HTTP log buffer instance, creating it if necessary
func GetHTTPLogBuffer() *LogBuffer {
	httpLogBufferOnce.Do(func() {
		httpLogBuffer = NewLogBuffer(1000) // Keep last 1000 HTTP log entries
	})
	return httpLogBuffer
}

// LogHTTPRequest logs an HTTP request to the separate HTTP log buffer
func LogHTTPRequest(method, path string, status int, duration time.Duration, size int, remoteAddr, userAgent, website, referer string, err error) {
	// Format timestamp in nginx style: [02/Jan/2006:15:04:05 -0700]
	timestamp := time.Now().Format("02/Jan/2006:15:04:05 -0700")
	
	// Format request line
	request := fmt.Sprintf("%s %s HTTP/1.1", method, path)
	
	// Use "-" for empty referer
	if referer == "" {
		referer = "-"
	}
	
	// Format message in nginx access log style
	// $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
	message := fmt.Sprintf(`%s - - [%s] "%s" %d %d "%s" "%s"`,
		remoteAddr,
		timestamp,
		request,
		status,
		size,
		referer,
		userAgent,
	)
	
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   message,
		Fields: map[string]any{
			"method":      method,
			"path":        path,
			"status":      status,
			"duration_ms": duration.Milliseconds(),
			"size":        size,
			"remote_addr": remoteAddr,
			"user_agent":  userAgent,
			"referer":     referer,
		},
	}

	if website != "" {
		entry.Fields["website"] = website
	}

	if err != nil {
		entry.Level = "error"
		entry.Fields["error"] = err.Error()
	}

	// Add to HTTP log buffer
	httpLogBuffer := GetHTTPLogBuffer()
	httpLogBuffer.AddEntry(entry)
}