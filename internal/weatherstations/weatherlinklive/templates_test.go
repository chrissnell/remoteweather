package weatherlinklive

import (
	"testing"
)

func TestGetTemplate(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantNil  bool
		wantName string
	}{
		{
			name:     "VP2 Basic exists",
			id:       "vp2_basic",
			wantNil:  false,
			wantName: "Vantage Pro2 or Vantage Vue",
		},
		{
			name:     "VP2 Plus exists",
			id:       "vp2_plus",
			wantNil:  false,
			wantName: "Vantage Pro2 Plus",
		},
		{
			name:     "VP2 Plus Split Wind exists",
			id:       "vp2_plus_split_wind",
			wantNil:  false,
			wantName: "Vantage Pro2 Plus with Additional Anemometer",
		},
		{
			name:     "VP2 Plus Agricultural exists",
			id:       "vp2_plus_agricultural",
			wantNil:  false,
			wantName: "Vantage Pro2 Plus with Soil/Leaf Station",
		},
		{
			name:    "Non-existent template",
			id:      "nonexistent",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetTemplate(tt.id)
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetTemplate(%q) = %v, want nil", tt.id, got)
				}
			} else {
				if got == nil {
					t.Errorf("GetTemplate(%q) = nil, want template", tt.id)
					return
				}
				if got.Name != tt.wantName {
					t.Errorf("GetTemplate(%q).Name = %q, want %q", tt.id, got.Name, tt.wantName)
				}
			}
		})
	}
}

func TestListTemplates(t *testing.T) {
	templates := ListTemplates()

	if len(templates) != 4 {
		t.Errorf("ListTemplates() returned %d templates, want 4", len(templates))
	}

	// Check all required templates exist
	required := map[string]bool{
		"vp2_basic":             false,
		"vp2_plus":              false,
		"vp2_plus_split_wind":   false,
		"vp2_plus_agricultural": false,
	}

	for _, tmpl := range templates {
		if _, exists := required[tmpl.ID]; exists {
			required[tmpl.ID] = true
		}
	}

	for id, found := range required {
		if !found {
			t.Errorf("Required template %q not found in ListTemplates()", id)
		}
	}
}

func TestTemplateIDs(t *testing.T) {
	ids := TemplateIDs()

	if len(ids) != 4 {
		t.Errorf("TemplateIDs() returned %d IDs, want 4", len(ids))
	}

	expected := map[string]bool{
		"vp2_basic":             false,
		"vp2_plus":              false,
		"vp2_plus_split_wind":   false,
		"vp2_plus_agricultural": false,
	}

	for _, id := range ids {
		if _, exists := expected[id]; exists {
			expected[id] = true
		}
	}

	for id, found := range expected {
		if !found {
			t.Errorf("Expected template ID %q not found in TemplateIDs()", id)
		}
	}
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    *Template
		wantErr bool
	}{
		{
			name:    "Nil template",
			tmpl:    nil,
			wantErr: false,
		},
		{
			name:    "VP2 Basic valid",
			tmpl:    GetTemplate("vp2_basic"),
			wantErr: false,
		},
		{
			name:    "VP2 Plus valid",
			tmpl:    GetTemplate("vp2_plus"),
			wantErr: false,
		},
		{
			name:    "VP2 Plus Split Wind valid",
			tmpl:    GetTemplate("vp2_plus_split_wind"),
			wantErr: false,
		},
		{
			name:    "VP2 Plus Agricultural valid",
			tmpl:    GetTemplate("vp2_plus_agricultural"),
			wantErr: false,
		},
		{
			name: "Invalid mapping string",
			tmpl: &Template{
				ID:            "invalid",
				Name:          "Invalid",
				MappingString: "invalid:mapping:string:too:many:parts",
			},
			wantErr: true, // Parser will error on invalid txid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTemplate(tt.tmpl)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSuggestTemplate(t *testing.T) {
	tests := []struct {
		name       string
		conditions []Condition
		want       string
	}{
		{
			name:       "Empty conditions",
			conditions: []Condition{},
			want:       "custom",
		},
		{
			name: "No TX1",
			conditions: []Condition{
				{TxID: intPtr(2), Temp: float64Ptr(72.5)},
			},
			want: "custom",
		},
		{
			name: "VP2 Basic - TX1 with temp, hum, wind, rain",
			conditions: []Condition{
				{
					TxID:          intPtr(1),
					Temp:          float64Ptr(72.5),
					Humidity:      intPtr(65),
					WindSpeedLast: float64Ptr(5.0),
					RainfallDaily: float64Ptr(0.5),
				},
			},
			want: "vp2_basic",
		},
		{
			name: "VP2 Plus - TX1 with all sensors including solar and UV",
			conditions: []Condition{
				{
					TxID:          intPtr(1),
					Temp:          float64Ptr(72.5),
					Humidity:      intPtr(65),
					WindSpeedLast: float64Ptr(5.0),
					RainfallDaily: float64Ptr(0.5),
					SolarRad:      intPtr(800),
					UVIndex:       float64Ptr(5.5),
				},
			},
			want: "vp2_plus",
		},
		{
			name: "VP2 Plus Split Wind - TX1 main, TX2 wind",
			conditions: []Condition{
				{
					TxID:          intPtr(1),
					Temp:          float64Ptr(72.5),
					Humidity:      intPtr(65),
					RainfallDaily: float64Ptr(0.5),
					SolarRad:      intPtr(800),
					UVIndex:       float64Ptr(5.5),
				},
				{
					TxID:          intPtr(2),
					WindSpeedLast: float64Ptr(5.0),
				},
			},
			want: "vp2_plus_split_wind",
		},
		{
			name: "VP2 Plus Agricultural - TX1 main, TX2 soil/leaf",
			conditions: []Condition{
				{
					TxID:          intPtr(1),
					Temp:          float64Ptr(72.5),
					Humidity:      intPtr(65),
					WindSpeedLast: float64Ptr(5.0),
					RainfallDaily: float64Ptr(0.5),
					SolarRad:      intPtr(800),
					UVIndex:       float64Ptr(5.5),
				},
				{
					TxID:        intPtr(2),
					Temp1:       float64Ptr(65.0),
					Temp2:       float64Ptr(66.0),
					MoistSoil1:  intPtr(50),
					MoistSoil2:  intPtr(55),
					WetLeaf1:    intPtr(5),
				},
			},
			want: "vp2_plus_agricultural",
		},
		{
			name: "Custom - TX1 only with temp",
			conditions: []Condition{
				{
					TxID: intPtr(1),
					Temp: float64Ptr(72.5),
				},
			},
			want: "custom",
		},
		{
			name: "Custom - TX1 with unusual sensor combination",
			conditions: []Condition{
				{
					TxID:     intPtr(1),
					Temp:     float64Ptr(72.5),
					UVIndex:  float64Ptr(5.5),
				},
			},
			want: "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SuggestTemplate(tt.conditions)
			if got != tt.want {
				t.Errorf("SuggestTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTemplateMappingStrings(t *testing.T) {
	// Verify all templates have valid mapping strings that can be parsed
	templates := ListTemplates()

	for _, tmpl := range templates {
		t.Run(tmpl.ID, func(t *testing.T) {
			mappings, err := ParseMappingString(tmpl.MappingString)
			if err != nil {
				t.Errorf("ParseMappingString(%q) error = %v", tmpl.MappingString, err)
				return
			}

			if len(mappings) == 0 {
				t.Errorf("ParseMappingString(%q) returned 0 mappings", tmpl.MappingString)
			}

			// Verify all required transmitter IDs appear in mappings
			foundTxIDs := make(map[int]bool)
			for _, m := range mappings {
				if m.TxID != nil {
					foundTxIDs[*m.TxID] = true
				}
			}

			for _, requiredTxID := range tmpl.RequiredTxIDs {
				if !foundTxIDs[requiredTxID] {
					t.Errorf("Template %q missing required TX ID %d in mappings", tmpl.ID, requiredTxID)
				}
			}
		})
	}
}

func TestTemplateRequiredFields(t *testing.T) {
	// Ensure all templates have required fields populated
	templates := ListTemplates()

	for _, tmpl := range templates {
		t.Run(tmpl.ID, func(t *testing.T) {
			if tmpl.ID == "" {
				t.Error("Template ID is empty")
			}
			if tmpl.Name == "" {
				t.Error("Template Name is empty")
			}
			if tmpl.Description == "" {
				t.Error("Template Description is empty")
			}
			if tmpl.MappingString == "" {
				t.Error("Template MappingString is empty")
			}
			if len(tmpl.RequiredTxIDs) == 0 {
				t.Error("Template RequiredTxIDs is empty")
			}
			if len(tmpl.RequiredSensors) == 0 {
				t.Error("Template RequiredSensors is empty")
			}
		})
	}
}

// Helper functions for creating pointers in test data
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}
