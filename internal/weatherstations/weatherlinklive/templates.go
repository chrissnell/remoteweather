package weatherlinklive

// Template represents a predefined sensor configuration for common Davis weather stations
type Template struct {
	ID                  string
	Name                string
	Description         string
	MappingString       string
	RequiredTxIDs       []int
	RequiredSensors     map[int][]string
	Notes               string
}

// Templates contains all predefined sensor configurations
var Templates = []Template{
	{
		ID:            "vp2_basic",
		Name:          "Vantage Pro2 or Vantage Vue",
		Description:   "Basic configuration for factory-default Vantage Pro2 or Vantage Vue. All sensors connected to main ISS transmitter on ID 1.",
		MappingString: "th:1, rain:1, wind:1, windchill:1, thw:1:appTemp, th_indoor, baro, battery:1",
		RequiredTxIDs: []int{1},
		RequiredSensors: map[int][]string{
			1: {"temp", "hum", "wind", "rain"},
		},
	},
	{
		ID:            "vp2_plus",
		Name:          "Vantage Pro2 Plus",
		Description:   "Vantage Pro2 Plus with solar radiation and UV sensors. All sensors on main ISS transmitter ID 1.",
		MappingString: "th:1, rain:1, wind:1, uv:1, solar:1, windchill:1, thw:1, thsw:1:appTemp, th_indoor, baro, battery:1",
		RequiredTxIDs: []int{1},
		RequiredSensors: map[int][]string{
			1: {"temp", "hum", "wind", "rain", "solar_rad", "uv_index"},
		},
	},
	{
		ID:            "vp2_plus_split_wind",
		Name:          "Vantage Pro2 Plus with Additional Anemometer",
		Description:   "VP2 Plus configuration with wind sensor on separate transmitter ID 2.",
		MappingString: "th:1, rain:1, wind:2, uv:1, solar:1, windchill:1, thw:1, thsw:1:appTemp, th_indoor, baro, battery:1, battery:2",
		RequiredTxIDs: []int{1, 2},
		RequiredSensors: map[int][]string{
			1: {"temp", "hum", "rain", "solar_rad", "uv_index"},
			2: {"wind"},
		},
		Notes: "Requires WeatherLink.com configuration to import wind from TX2 to TX1 for compound indices (windchill, thw, thsw). If wind import is NOT configured, remove windchill, thw, and thsw from mapping.",
	},
	{
		ID:            "vp2_plus_agricultural",
		Name:          "Vantage Pro2 Plus with Soil/Leaf Station",
		Description:   "VP2 Plus with full agricultural monitoring station on transmitter ID 2.",
		MappingString: "th:1, rain:1, wind:1, uv:1, solar:1, windchill:1, thw:1, thsw:1:appTemp, soil_temp:2:1, soil_temp:2:2, soil_temp:2:3, soil_temp:2:4, soil_moist:2:1, soil_moist:2:2, soil_moist:2:3, soil_moist:2:4, leaf_wet:2:1, leaf_wet:2:2, th_indoor, baro, battery:1, battery:2",
		RequiredTxIDs: []int{1, 2},
		RequiredSensors: map[int][]string{
			1: {"temp", "hum", "wind", "rain", "solar_rad", "uv_index"},
			2: {"soil_temp", "soil_moist", "leaf_wet"},
		},
	},
}

// GetTemplate returns a template by ID, or nil if not found
func GetTemplate(id string) *Template {
	for i := range Templates {
		if Templates[i].ID == id {
			return &Templates[i]
		}
	}
	return nil
}

// ListTemplates returns all available templates
func ListTemplates() []Template {
	return Templates
}

// TemplateIDs returns a slice of all template IDs
func TemplateIDs() []string {
	ids := make([]string, len(Templates))
	for i, t := range Templates {
		ids[i] = t.ID
	}
	return ids
}

// ValidateTemplate checks if a template's mapping string is valid
func ValidateTemplate(template *Template) error {
	if template == nil {
		return nil
	}

	_, err := ParseMappingString(template.MappingString)
	return err
}

// SuggestTemplate suggests a template based on discovered transmitters and sensors
// Returns the template ID or "custom" if no template matches
func SuggestTemplate(conditions []Condition) string {
	// Build transmitter capability map
	transmitters := make(map[int]map[string]bool)

	for _, condition := range conditions {
		if condition.TxID == nil {
			continue
		}

		txid := *condition.TxID
		if transmitters[txid] == nil {
			transmitters[txid] = make(map[string]bool)
		}

		// Check for various sensor types
		if condition.Temp != nil {
			transmitters[txid]["temp"] = true
		}
		if condition.Humidity != nil {
			transmitters[txid]["hum"] = true
		}
		if condition.WindSpeedLast != nil {
			transmitters[txid]["wind"] = true
		}
		if condition.RainfallDaily != nil || condition.RainRateLast != nil {
			transmitters[txid]["rain"] = true
		}
		if condition.SolarRad != nil {
			transmitters[txid]["solar_rad"] = true
		}
		if condition.UVIndex != nil {
			transmitters[txid]["uv_index"] = true
		}
		if condition.Temp1 != nil || condition.Temp2 != nil || condition.Temp3 != nil || condition.Temp4 != nil {
			transmitters[txid]["soil_temp"] = true
		}
		if condition.MoistSoil1 != nil || condition.MoistSoil2 != nil || condition.MoistSoil3 != nil || condition.MoistSoil4 != nil {
			transmitters[txid]["soil_moist"] = true
		}
		if condition.WetLeaf1 != nil || condition.WetLeaf2 != nil {
			transmitters[txid]["leaf_wet"] = true
		}
	}

	// Find main ISS (TX1)
	tx1, hasTx1 := transmitters[1]
	if !hasTx1 {
		return "custom" // No TX1, can't use templates
	}

	// Single transmitter scenarios
	if len(transmitters) == 1 {
		if tx1["solar_rad"] && tx1["uv_index"] {
			return "vp2_plus"
		}
		if tx1["wind"] && tx1["rain"] {
			return "vp2_basic"
		}
	}

	// Multi-transmitter scenarios
	if len(transmitters) >= 2 {
		tx2, hasTx2 := transmitters[2]
		if hasTx2 {
			// Check if TX2 has agricultural sensors
			if tx2["soil_temp"] || tx2["soil_moist"] || tx2["leaf_wet"] {
				return "vp2_plus_agricultural"
			}

			// Check if TX2 has wind (split anemometer)
			if tx2["wind"] {
				return "vp2_plus_split_wind"
			}
		}
	}

	// Default to custom if no template matches
	return "custom"
}
