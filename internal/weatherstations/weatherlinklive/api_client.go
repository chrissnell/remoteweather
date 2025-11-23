package weatherlinklive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GetCurrentConditions retrieves current weather conditions from WLL device via HTTP
func GetCurrentConditions(ctx context.Context, host string) (*CurrentConditionsResponse, error) {
	url := fmt.Sprintf("http://%s/v1/current_conditions", host)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to device: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result CurrentConditionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("API error %d: %s", result.Error.Code, result.Error.Message)
	}

	return &result, nil
}

// StartRealTimeBroadcast starts UDP broadcast mode on the WLL device
func StartRealTimeBroadcast(ctx context.Context, host string, duration int) (*RealTimeResponse, error) {
	url := fmt.Sprintf("http://%s/v1/real_time?duration=%d", host, duration)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to device: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result RealTimeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("API error %d: %s", result.Error.Code, result.Error.Message)
	}

	return &result, nil
}
