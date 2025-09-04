package responseformat

import (
	"encoding/json"
	"net/http"

	"github.com/vmihailenco/msgpack/v5"
)

// Formatter handles encoding and writing responses in JSON or MessagePack format
type Formatter struct{}

// NewFormatter creates a new response formatter
func NewFormatter() *Formatter {
	return &Formatter{}
}

// WriteResponse writes the response in the appropriate format based on the query parameter
// JSON is the default format. MessagePack is used when format=msgpack is specified
func (f *Formatter) WriteResponse(w http.ResponseWriter, req *http.Request, data any, headers map[string]string) error {
	// Set any provided headers first
	for k, v := range headers {
		w.Header().Set(k, v)
	}

	// Always set CORS header
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check if MessagePack is requested via format=msgpack query parameter
	if req.URL.Query().Get("format") == "msgpack" {
		return f.writeMsgPack(w, data)
	}

	// Default to JSON format (when no format parameter or any other value)
	return f.writeJSON(w, data)
}

// WriteRawJSON writes pre-encoded JSON data with optional wrapper
func (f *Formatter) WriteRawJSON(w http.ResponseWriter, req *http.Request, jsonBytes []byte, wrapper *JSONWrapper) error {
	// Always set CORS header
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Check if MessagePack is requested
	if req.URL.Query().Get("format") == "msgpack" {
		// Need to decode JSON then encode as MessagePack
		var data any
		if wrapper != nil {
			// Build the wrapped structure
			wrapped := map[string]any{
				"lastUpdated": wrapper.LastUpdated,
			}
			if err := json.Unmarshal(jsonBytes, &data); err != nil {
				return err
			}
			wrapped["data"] = data
			return f.writeMsgPack(w, wrapped)
		}
		// Direct conversion
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return err
		}
		return f.writeMsgPack(w, data)
	}

	// Default to JSON
	w.Header().Set("Content-Type", "application/json")
	if wrapper != nil {
		// Write wrapped JSON
		w.Write([]byte(`{"lastUpdated": "` + wrapper.LastUpdated + `", "data": `))
		w.Write(jsonBytes)
		w.Write([]byte("}"))
	} else {
		w.Write(jsonBytes)
	}
	return nil
}

func (f *Formatter) writeJSON(w http.ResponseWriter, data any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(data)
}

func (f *Formatter) writeMsgPack(w http.ResponseWriter, data any) error {
	w.Header().Set("Content-Type", "application/x-msgpack")
	encoder := msgpack.NewEncoder(w)
	encoder.SetCustomStructTag("json") // Use json tags for MessagePack
	return encoder.Encode(data)
}

// JSONWrapper is used for wrapping raw JSON data with metadata
type JSONWrapper struct {
	LastUpdated string
}