package weatherlinklive

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseMappingString parses a mapping configuration string into SensorMapping structs
// Format: "type:txid:port_or_option, type:txid, ..."
// Examples:
//   - "th:1" - Temperature/humidity on transmitter 1
//   - "wind:1" - Wind sensor on transmitter 1
//   - "soil_temp:2:3" - Soil temp sensor on transmitter 2, port 3
//   - "thsw:1:appTemp" - THSW index on TX1, also map to apparent temp
func ParseMappingString(mappingStr string) ([]SensorMapping, error) {
	if mappingStr == "" {
		return nil, fmt.Errorf("mapping string is empty")
	}

	parts := strings.Split(mappingStr, ",")
	mappings := make([]SensorMapping, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		components := strings.Split(part, ":")
		if len(components) < 1 {
			return nil, fmt.Errorf("invalid mapping: %s", part)
		}

		mapping := SensorMapping{
			Type: strings.TrimSpace(components[0]),
		}

		// Parse txid (if present and not empty)
		if len(components) > 1 && components[1] != "" {
			txid, err := strconv.Atoi(strings.TrimSpace(components[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid txid in %s: %w", part, err)
			}
			mapping.TxID = &txid
		}

		// Parse port or options (if present)
		if len(components) > 2 {
			// For soil/leaf sensors, component[2] is a port number
			if mapping.Type == "soil_temp" || mapping.Type == "soil_moist" || mapping.Type == "leaf_wet" {
				port, err := strconv.Atoi(strings.TrimSpace(components[2]))
				if err != nil {
					return nil, fmt.Errorf("invalid port in %s: %w", part, err)
				}
				if port < 1 || port > 4 {
					return nil, fmt.Errorf("port must be 1-4 in %s", part)
				}
				mapping.Port = &port
			} else {
				// Otherwise, remaining components are options
				options := make([]string, 0, len(components)-2)
				for i := 2; i < len(components); i++ {
					opt := strings.TrimSpace(components[i])
					if opt != "" {
						options = append(options, opt)
					}
				}
				mapping.Options = options
			}
		}

		mappings = append(mappings, mapping)
	}

	if len(mappings) == 0 {
		return nil, fmt.Errorf("no valid mappings found in string: %s", mappingStr)
	}

	return mappings, nil
}
