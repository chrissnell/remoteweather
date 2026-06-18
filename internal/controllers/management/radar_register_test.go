package management

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterRadarToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/register/remoteweather" || r.Method != http.MethodPost {
			http.Error(w, "bad route", http.StatusNotFound)
			return
		}
		var body struct {
			Instance           string `json:"instance"`
			AgreeNoncommercial bool   `json:"agree_noncommercial"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Instance != "suncrestweather.com" || !body.AgreeNoncommercial {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"instance": body.Instance, "token": "tok-xyz"})
	}))
	defer srv.Close()

	old := radarAuthBaseURL
	radarAuthBaseURL = srv.URL
	defer func() { radarAuthBaseURL = old }()

	token, err := registerRadarToken("suncrestweather.com")
	if err != nil {
		t.Fatalf("registerRadarToken: %v", err)
	}
	if token != "tok-xyz" {
		t.Errorf("token = %q, want tok-xyz", token)
	}
}

func TestRegisterRadarTokenUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "device_limit_reached"})
	}))
	defer srv.Close()
	old := radarAuthBaseURL
	radarAuthBaseURL = srv.URL
	defer func() { radarAuthBaseURL = old }()

	if _, err := registerRadarToken("suncrestweather.com"); err == nil {
		t.Fatal("expected error on non-200 upstream")
	}
}
