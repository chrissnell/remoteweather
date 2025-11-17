package weatherstations

import (
	"fmt"
	"math"

	"github.com/chrissnell/remoteweather/pkg/config"
	"go.uber.org/zap"
)

// CalculateWindChill calculates wind chill temperature using the NWS formula
// Returns 0 if wind chill doesn't apply (temp > 50°F or wind < 3 mph)
func CalculateWindChill(tempF, windSpeedMph float32) float32 {
	if tempF > 50 || windSpeedMph < 3 {
		return tempF
	}
	return 35.74 + 0.6215*tempF - 35.75*float32(math.Pow(float64(windSpeedMph), 0.16)) + 0.4275*tempF*float32(math.Pow(float64(windSpeedMph), 0.16))
}

// CalculateHeatIndex calculates heat index using the NWS formula
// Returns 0 if heat index doesn't apply (temp < 80°F)
func CalculateHeatIndex(tempF, humidity float32) float32 {
	if tempF < 80 {
		return tempF
	}

	c1 := float32(-42.379)
	c2 := float32(2.04901523)
	c3 := float32(10.14333127)
	c4 := float32(-0.22475541)
	c5 := float32(-0.00683783)
	c6 := float32(-0.05481717)
	c7 := float32(0.00122874)
	c8 := float32(0.00085282)
	c9 := float32(-0.00000199)

	return c1 + c2*tempF + c3*humidity + c4*tempF*humidity + c5*tempF*tempF +
		c6*humidity*humidity + c7*tempF*tempF*humidity + c8*tempF*humidity*humidity +
		c9*tempF*tempF*humidity*humidity
}

// LoadDeviceConfig loads configuration for a specific device
func LoadDeviceConfig(configProvider config.ConfigProvider, deviceName string, logger *zap.SugaredLogger) *config.DeviceData {
	cfgData, err := configProvider.LoadConfig()
	if err != nil {
		logger.Fatalf("Station [%s] failed to load config: %v", deviceName, err)
	}

	for _, device := range cfgData.Devices {
		if device.Name == deviceName {
			return &device
		}
	}

	logger.Fatalf("Station [%s] device not found in configuration", deviceName)
	return nil
}

// ValidateSerialOrNetwork validates that either serial device or network config is provided
func ValidateSerialOrNetwork(config config.DeviceData) error {
	if config.SerialDevice == "" && (config.Hostname == "" || config.Port == "") {
		return fmt.Errorf("station [%s] must define either a serial device or hostname+port", config.Name)
	}
	return nil
}
