package weatherlinklive

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ValidationIssue represents a single validation issue
type ValidationIssue struct {
	Severity string `json:"severity"` // "error", "warning", "info"
	Field    string `json:"field"`
	Message  string `json:"message"`
}

// ValidationResult contains the validation results for a mapping configuration
type ValidationResult struct {
	Valid     bool                `json:"valid"`
	Issues    []ValidationIssue   `json:"issues"`
	Warnings  int                 `json:"warnings"`
	Errors    int                 `json:"errors"`
	Mapping   *MappingValidation  `json:"mapping,omitempty"`
}

// MappingValidation contains validated mapping information
type MappingValidation struct {
	MappingString     string            `json:"mapping_string"`
	Mappings          []SensorMapping   `json:"mappings"`
	TransmittersUsed  []int             `json:"transmitters_used"`
	SensorTypes       map[string]int    `json:"sensor_types"` // count by type
	RequiresBroadcast bool              `json:"requires_broadcast"`
}

// ValidateMappingString validates a mapping string without device connectivity
func ValidateMappingString(mappingStr string) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Issues: make([]ValidationIssue, 0),
	}

	// Parse mapping string
	mappings, err := ParseMappingString(mappingStr)
	if err != nil {
		result.Valid = false
		result.Errors++
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: "error",
			Field:    "mapping_string",
			Message:  fmt.Sprintf("Failed to parse mapping string: %v", err),
		})
		return result
	}

	if len(mappings) == 0 {
		result.Valid = false
		result.Errors++
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: "error",
			Field:    "mapping_string",
			Message:  "No sensor mappings defined",
		})
		return result
	}

	// Build validation info
	result.Mapping = &MappingValidation{
		MappingString:    mappingStr,
		Mappings:         mappings,
		TransmittersUsed: make([]int, 0),
		SensorTypes:      make(map[string]int),
	}

	// Track transmitters
	txMap := make(map[int]bool)

	// Validate each mapping
	for i, mapping := range mappings {
		// Track transmitter usage
		if mapping.TxID != nil {
			txMap[*mapping.TxID] = true
		}

		// Count sensor types
		result.Mapping.SensorTypes[mapping.Type]++

		// Validate sensor type
		if !isValidSensorType(mapping.Type) {
			result.Warnings++
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: "warning",
				Field:    fmt.Sprintf("mappings[%d].type", i),
				Message:  fmt.Sprintf("Unknown sensor type: %s", mapping.Type),
			})
		}

		// Validate transmitter ID
		if mapping.TxID != nil && (*mapping.TxID < 1 || *mapping.TxID > 8) {
			result.Valid = false
			result.Errors++
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: "error",
				Field:    fmt.Sprintf("mappings[%d].tx_id", i),
				Message:  fmt.Sprintf("Transmitter ID must be between 1 and 8, got %d", *mapping.TxID),
			})
		}

		// Validate port numbers for soil/leaf sensors
		if mapping.Port != nil {
			if mapping.Type == "soil_temp" || mapping.Type == "soil_moist" || mapping.Type == "leaf_wet" {
				if *mapping.Port < 1 || *mapping.Port > 4 {
					result.Valid = false
					result.Errors++
					result.Issues = append(result.Issues, ValidationIssue{
						Severity: "error",
						Field:    fmt.Sprintf("mappings[%d].port", i),
						Message:  fmt.Sprintf("Port number must be between 1 and 4, got %d", *mapping.Port),
					})
				}
			}
		}

		// Validate options
		if len(mapping.Options) > 0 {
			for _, opt := range mapping.Options {
				if !isValidOption(mapping.Type, opt) {
					result.Warnings++
					result.Issues = append(result.Issues, ValidationIssue{
						Severity: "warning",
						Field:    fmt.Sprintf("mappings[%d].options", i),
						Message:  fmt.Sprintf("Unknown option '%s' for sensor type '%s'", opt, mapping.Type),
					})
				}
			}
		}
	}

	// Extract and sort transmitter IDs
	for txid := range txMap {
		result.Mapping.TransmittersUsed = append(result.Mapping.TransmittersUsed, txid)
	}
	sort.Ints(result.Mapping.TransmittersUsed)

	// Check if broadcast mode is required
	result.Mapping.RequiresBroadcast = requiresBroadcast(mappings)
	if result.Mapping.RequiresBroadcast {
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: "info",
			Field:    "broadcast_mode",
			Message:  "This configuration includes wind or rain sensors which require UDP broadcast mode for real-time updates",
		})
	}

	// Check for missing essential sensors
	if result.Mapping.SensorTypes["th"] == 0 {
		result.Warnings++
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: "warning",
			Field:    "sensors",
			Message:  "No temperature/humidity sensors configured - most weather stations should have at least one",
		})
	}

	return result
}

// ValidateMappings validates mappings against actual device conditions
func ValidateMappings(ctx context.Context, host string, mappings []SensorMapping) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Issues: make([]ValidationIssue, 0),
	}

	// Get current conditions from device
	resp, err := GetCurrentConditions(ctx, host)
	if err != nil {
		result.Valid = false
		result.Errors++
		result.Issues = append(result.Issues, ValidationIssue{
			Severity: "error",
			Field:    "device",
			Message:  fmt.Sprintf("Failed to connect to device: %v", err),
		})
		return result
	}

	// Build map of available sensors
	available := buildAvailabilityMap(resp.Data.Conditions)

	// Build validation info
	result.Mapping = &MappingValidation{
		Mappings:         mappings,
		TransmittersUsed: make([]int, 0),
		SensorTypes:      make(map[string]int),
	}

	// Track transmitters
	txMap := make(map[int]bool)

	// Validate each mapping against available sensors
	for i, mapping := range mappings {
		// Track transmitter usage
		if mapping.TxID != nil {
			txMap[*mapping.TxID] = true
		}

		// Count sensor types
		result.Mapping.SensorTypes[mapping.Type]++

		// Check if sensor is available on device
		isAvailable := checkSensorAvailability(available, mapping)
		if !isAvailable {
			result.Warnings++
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: "warning",
				Field:    fmt.Sprintf("mappings[%d]", i),
				Message:  fmt.Sprintf("Sensor not currently available on device: %s", describeSensorMapping(mapping)),
			})
		}
	}

	// Extract and sort transmitter IDs
	for txid := range txMap {
		result.Mapping.TransmittersUsed = append(result.Mapping.TransmittersUsed, txid)
	}
	sort.Ints(result.Mapping.TransmittersUsed)

	// Check if broadcast mode is required
	result.Mapping.RequiresBroadcast = requiresBroadcast(mappings)

	// Discover sensors to suggest additions
	discoveredSensors := DiscoverSensors(resp.Data.Conditions)
	suggestAdditionalSensors(result, mappings, discoveredSensors)

	return result
}

// SensorAvailability represents sensor availability on device
type SensorAvailability struct {
	TxID              *int
	Type              string
	Port              *int
	Available         bool
	DataStructureType int
}

// buildAvailabilityMap builds a map of available sensors from conditions
func buildAvailabilityMap(conditions []Condition) []SensorAvailability {
	availability := make([]SensorAvailability, 0)

	// Helper to create int pointer
	intPtr := func(i int) *int {
		return &i
	}

	for _, cond := range conditions {
		// Internal sensors
		if cond.TxID == nil {
			if cond.BarAbsolute != nil || cond.BarSeaLevel != nil {
				availability = append(availability, SensorAvailability{
					TxID:      nil,
					Type:      "baro",
					Available: true,
				})
			}
			if cond.TempIn != nil || cond.HumIn != nil {
				availability = append(availability, SensorAvailability{
					TxID:      nil,
					Type:      "th_indoor",
					Available: true,
				})
			}
			continue
		}

		// Transmitter sensors
		txid := cond.TxID

		// ISS sensors
		if cond.Temp != nil || cond.Humidity != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "th",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.WindSpeedLast != nil || cond.WindDirLast != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "wind",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.RainfallDaily != nil || cond.RainRateLast != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "rain",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.SolarRad != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "solar",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.UVIndex != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "uv",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.WindChill != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "windchill",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.THWIndex != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "thw",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.THSWIndex != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "thsw",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		// Agricultural sensors
		if cond.Temp1 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "soil_temp",
				Port:              intPtr(1),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}
		if cond.Temp2 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "soil_temp",
				Port:              intPtr(2),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}
		if cond.Temp3 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "soil_temp",
				Port:              intPtr(3),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}
		if cond.Temp4 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "soil_temp",
				Port:              intPtr(4),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.MoistSoil1 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "soil_moist",
				Port:              intPtr(1),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}
		if cond.MoistSoil2 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "soil_moist",
				Port:              intPtr(2),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}
		if cond.MoistSoil3 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "soil_moist",
				Port:              intPtr(3),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}
		if cond.MoistSoil4 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "soil_moist",
				Port:              intPtr(4),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		if cond.WetLeaf1 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "leaf_wet",
				Port:              intPtr(1),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}
		if cond.WetLeaf2 != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "leaf_wet",
				Port:              intPtr(2),
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}

		// Battery monitoring
		if cond.TransBatteryFlag != nil {
			availability = append(availability, SensorAvailability{
				TxID:              txid,
				Type:              "battery",
				Available:         true,
				DataStructureType: cond.DataStructureType,
			})
		}
	}

	return availability
}

// checkSensorAvailability checks if a sensor mapping is available on the device
func checkSensorAvailability(availability []SensorAvailability, mapping SensorMapping) bool {
	for _, avail := range availability {
		// Match type
		if avail.Type != mapping.Type {
			continue
		}

		// Match transmitter ID
		if !txidMatches(avail.TxID, mapping.TxID) {
			continue
		}

		// Match port (for soil/leaf sensors)
		if mapping.Port != nil && avail.Port != nil {
			if *mapping.Port != *avail.Port {
				continue
			}
		}

		// Found match
		return true
	}

	return false
}

// txidMatches checks if two transmitter IDs match
func txidMatches(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// isValidSensorType checks if a sensor type is recognized
func isValidSensorType(sensorType string) bool {
	validTypes := []string{
		"th", "wind", "rain", "solar", "uv", "windchill", "thw", "thsw",
		"baro", "th_indoor", "soil_temp", "soil_moist", "leaf_wet", "battery",
	}

	for _, valid := range validTypes {
		if sensorType == valid {
			return true
		}
	}

	return false
}

// isValidOption checks if an option is valid for a sensor type
func isValidOption(sensorType string, option string) bool {
	validOptions := map[string][]string{
		"thw":  {"appTemp"},
		"thsw": {"appTemp"},
	}

	options, exists := validOptions[sensorType]
	if !exists {
		return false
	}

	for _, valid := range options {
		if option == valid {
			return true
		}
	}

	return false
}

// requiresBroadcast checks if any mappings require UDP broadcast mode
func requiresBroadcast(mappings []SensorMapping) bool {
	for _, mapping := range mappings {
		if mapping.Type == "wind" || mapping.Type == "rain" {
			return true
		}
	}
	return false
}

// describeSensorMapping creates a human-readable description of a sensor mapping
func describeSensorMapping(mapping SensorMapping) string {
	parts := []string{mapping.Type}

	if mapping.TxID != nil {
		parts = append(parts, fmt.Sprintf("TX%d", *mapping.TxID))
	}

	if mapping.Port != nil {
		parts = append(parts, fmt.Sprintf("Port %d", *mapping.Port))
	}

	if len(mapping.Options) > 0 {
		parts = append(parts, fmt.Sprintf("(%s)", strings.Join(mapping.Options, ",")))
	}

	return strings.Join(parts, " ")
}

// suggestAdditionalSensors suggests sensors that are available but not configured
func suggestAdditionalSensors(result *ValidationResult, mappings []SensorMapping, discovered *DiscoveryResult) {
	// Build set of configured sensors
	configured := make(map[string]bool)
	for _, mapping := range mappings {
		key := fmt.Sprintf("%s:%v:%v", mapping.Type, mapping.TxID, mapping.Port)
		configured[key] = true
	}

	// Check for unconfigured sensors on transmitters
	for _, tx := range discovered.Transmitters {
		for _, sensor := range tx.Sensors {
			key := fmt.Sprintf("%s:%v:%v", sensor.Type, sensor.TxID, nil)
			if !configured[key] && sensor.Available {
				result.Issues = append(result.Issues, ValidationIssue{
					Severity: "info",
					Field:    "sensors",
					Message:  fmt.Sprintf("Available sensor not configured: %s", sensor.Description),
				})
			}
		}
	}

	// Check for unconfigured internal sensors
	for _, sensor := range discovered.InternalSensors {
		key := fmt.Sprintf("%s:%v:%v", sensor.Type, nil, nil)
		if !configured[key] && sensor.Available {
			result.Issues = append(result.Issues, ValidationIssue{
				Severity: "info",
				Field:    "sensors",
				Message:  fmt.Sprintf("Available sensor not configured: %s", sensor.Description),
			})
		}
	}
}
