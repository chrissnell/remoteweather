package management

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// radarAuthBaseURL is the Graywolf auth worker base. Package var so tests can
// redirect it to an httptest server.
var radarAuthBaseURL = "https://auth.nw5w.com"

type radarRegisterRequest struct {
	Instance           string `json:"instance"`
	AgreeNoncommercial bool   `json:"agree_noncommercial"`
}

type radarRegisterResponse struct {
	Instance string `json:"instance"`
	Token    string `json:"token"`
	Error    string `json:"error"`
}

// registerRadarToken registers a website hostname with the Graywolf auth worker
// (recording the non-commercial agreement) and returns the issued tile token.
func registerRadarToken(hostname string) (string, error) {
	payload, err := json.Marshal(radarRegisterRequest{Instance: hostname, AgreeNoncommercial: true})
	if err != nil {
		return "", fmt.Errorf("encoding radar registration request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, radarAuthBaseURL+"/register/remoteweather", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("creating radar registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling radar registration: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var parsed radarRegisterResponse
	_ = json.Unmarshal(bodyBytes, &parsed)

	if resp.StatusCode != http.StatusOK {
		if parsed.Error != "" {
			return "", fmt.Errorf("radar registration failed: %s (HTTP %d)", parsed.Error, resp.StatusCode)
		}
		return "", fmt.Errorf("radar registration failed: HTTP %d", resp.StatusCode)
	}
	if parsed.Token == "" {
		return "", fmt.Errorf("radar registration returned no token")
	}
	return parsed.Token, nil
}
