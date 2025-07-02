package aprs

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/chrissnell/remoteweather/internal/log"
	"github.com/chrissnell/remoteweather/pkg/aprs"
	"github.com/chrissnell/remoteweather/pkg/config"
)

// startHealthMonitor starts a goroutine that periodically updates the health status
func (a *Storage) startHealthMonitor(ctx context.Context, configProvider config.ConfigProvider) {
	go func() {
		// Run initial health check immediately
		a.updateHealthStatus(configProvider)

		ticker := time.NewTicker(90 * time.Second) // Update health every 90 seconds (less frequent due to network calls)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.updateHealthStatus(configProvider)
			case <-ctx.Done():
				log.Info("stopping APRS health monitor")
				return
			}
		}
	}()
}

// updateHealthStatus performs a health check and updates the status in the config database
func (a *Storage) updateHealthStatus(configProvider config.ConfigProvider) {
	health := &config.StorageHealthData{
		LastCheck: time.Now(),
		Status:    "healthy",
		Message:   "APRS-IS connection available",
	}

	// Test APRS-IS server connectivity and authentication
	err := a.testAPRSISLogin(configProvider)
	if err != nil {
		health.Status = "unhealthy"
		health.Message = "APRS-IS login test failed"
		health.Error = err.Error()
	} else {
		health.Status = "healthy"
		health.Message = "APRS-IS login test successful"
	}

	// Update health status in configuration database
	err = configProvider.UpdateStorageHealth("aprs", health)
	if err != nil {
		log.Errorf("Failed to update APRS health status: %v", err)
	} else {
		log.Infof("Updated APRS health status: %s", health.Status)
	}
}

// testAPRSISLogin performs a test login to the APRS-IS server to verify connectivity and authentication
func (a *Storage) testAPRSISLogin(configProvider config.ConfigProvider) error {
	connectionTimeout := 10 * time.Second

	// Load APRS storage configuration
	storageConfig, err := configProvider.GetStorageConfig()
	if err != nil {
		return fmt.Errorf("error loading storage configuration: %v", err)
	}

	if storageConfig.APRS == nil || storageConfig.APRS.Server == "" {
		return fmt.Errorf("APRS storage configuration is missing or incomplete")
	}

	// Get station APRS configurations
	stationConfigs, err := configProvider.GetStationAPRSConfigs()
	if err != nil {
		return fmt.Errorf("error loading station APRS configurations: %v", err)
	}

	var stationConfig *config.StationAPRSData
	for _, station := range stationConfigs {
		if station.Enabled && station.Callsign != "" {
			stationConfig = &station
			break
		}
	}

	if stationConfig == nil {
		return fmt.Errorf("no enabled station APRS configuration found")
	}

	// Test connection to APRS-IS server
	dialer := net.Dialer{
		Timeout: connectionTimeout,
	}

	conn, err := dialer.Dial("tcp", storageConfig.APRS.Server)
	if err != nil {
		return fmt.Errorf("failed to connect to APRS-IS server %s: %v", storageConfig.APRS.Server, err)
	}
	defer conn.Close()

	buffCon := bufio.NewReader(conn)

	// Set read deadline for server greeting
	conn.SetReadDeadline(time.Now().Add(connectionTimeout))

	// Read server greeting
	resp, err := buffCon.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read APRS-IS server greeting: %v", err)
	}

	// Verify proper greeting format
	if len(resp) == 0 || resp[0] != '#' {
		return fmt.Errorf("APRS-IS server responded with invalid greeting: %s", strings.TrimSpace(resp))
	}

	// Calculate passcode from callsign
	passcode := aprs.CalculatePasscode(stationConfig.Callsign)

	// Send login command
	loginCmd := fmt.Sprintf("user %s pass %d vers remoteweather-healthcheck 1.0\r\n",
		stationConfig.Callsign, passcode)

	conn.SetWriteDeadline(time.Now().Add(connectionTimeout))
	_, err = conn.Write([]byte(loginCmd))
	if err != nil {
		return fmt.Errorf("failed to send login command: %v", err)
	}

	// Read login response
	conn.SetReadDeadline(time.Now().Add(connectionTimeout))
	loginResp, err := buffCon.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read login response: %v", err)
	}

	// Check if login was successful
	// APRS-IS typically responds with a line starting with '#' containing "verified" for successful logins
	loginResp = strings.TrimSpace(loginResp)
	if !strings.Contains(strings.ToLower(loginResp), "verified") {
		return fmt.Errorf("APRS-IS login failed, server response: %s", loginResp)
	}

	log.Debugf("APRS-IS health check successful for callsign %s", stationConfig.Callsign)
	return nil
}
