package grpcreceiver

import (
	"fmt"
	"sync"
	"time"

	"github.com/chrissnell/remoteweather/pkg/config"
	pb "github.com/chrissnell/remoteweather/protocols/remoteweather"
	"go.uber.org/zap"
)

// RemoteStation represents a registered remote station in memory
type RemoteStation struct {
	StationID   string
	StationName string
	StationType string
	APRS        *APRSConfig
	WU          *WUConfig
	Aeris       *AerisConfig
	PWS         *PWSConfig
	LastSeen    time.Time
}

// Service configurations
type APRSConfig struct {
	Enabled  bool
	Callsign string
	Password string
}

type WUConfig struct {
	Enabled   bool
	StationID string
	APIKey    string
}

type AerisConfig struct {
	Enabled      bool
	ClientID     string
	ClientSecret string
}

type PWSConfig struct {
	Enabled   bool
	StationID string
	Password  string
}

// RemoteStationRegistry manages registered remote stations
type RemoteStationRegistry struct {
	mu             sync.RWMutex
	configProvider config.ConfigProvider
	stations       map[string]*RemoteStation // UUID -> station (in-memory cache)
	logger         *zap.SugaredLogger
}

// NewRemoteStationRegistry creates a new registry using the config provider
func NewRemoteStationRegistry(configProvider config.ConfigProvider, logger *zap.SugaredLogger) (*RemoteStationRegistry, error) {
	registry := &RemoteStationRegistry{
		configProvider: configProvider,
		stations:       make(map[string]*RemoteStation),
		logger:         logger,
	}

	// Load existing stations from database via config provider
	if err := registry.loadStations(); err != nil {
		return nil, fmt.Errorf("failed to load remote stations: %w", err)
	}

	return registry, nil
}

// loadStations loads all remote stations from the database into memory
func (r *RemoteStationRegistry) loadStations() error {
	// Get SQLite provider from cached provider
	cachedProvider, ok := r.configProvider.(*config.CachedConfigProvider)
	if !ok {
		r.logger.Debug("config provider is not cached, cannot load remote stations")
		return nil
	}
	
	sqliteProvider, ok := cachedProvider.GetUnderlying().(*config.SQLiteProvider)
	if !ok {
		r.logger.Debug("underlying provider is not SQLite, cannot load remote stations")
		return nil
	}
	
	// Load stations from config provider
	stations, err := sqliteProvider.GetRemoteStations()
	if err != nil {
		r.logger.Warnf("failed to load remote stations: %v", err)
		// Not a fatal error - table might not exist yet
		return nil
	}

	// Convert to in-memory representation
	for _, station := range stations {
		remoteStation := &RemoteStation{
			StationID:   station.StationID,
			StationName: station.StationName,
			StationType: station.StationType,
			LastSeen:    station.LastSeen,
		}

		// Build service configs
		if station.APRSEnabled {
			remoteStation.APRS = &APRSConfig{
				Enabled:  true,
				Callsign: station.APRSCallsign,
				Password: station.APRSPassword,
			}
		}

		if station.WUEnabled {
			remoteStation.WU = &WUConfig{
				Enabled:   true,
				StationID: station.WUStationID,
				APIKey:    station.WUAPIKey,
			}
		}

		if station.AerisEnabled {
			remoteStation.Aeris = &AerisConfig{
				Enabled:      true,
				ClientID:     station.AerisClientID,
				ClientSecret: station.AerisClientSecret,
			}
		}

		if station.PWSEnabled {
			remoteStation.PWS = &PWSConfig{
				Enabled:   true,
				StationID: station.PWSStationID,
				Password:  station.PWSPassword,
			}
		}

		r.stations[remoteStation.StationID] = remoteStation
	}

	r.logger.Infof("Loaded %d remote stations from database", len(r.stations))
	return nil
}

// Register registers a new remote station or updates an existing one
func (r *RemoteStationRegistry) Register(pbConfig *pb.RemoteStationConfig) (string, error) {
	// Convert protobuf config to config data
	configData := &config.RemoteStationData{
		StationID:         pbConfig.StationId,
		StationName:       pbConfig.StationName,
		StationType:       pbConfig.StationType,
		APRSEnabled:       pbConfig.AprsEnabled,
		APRSCallsign:      pbConfig.AprsCallsign,
		APRSPassword:      pbConfig.AprsPassword,
		WUEnabled:         pbConfig.WuEnabled,
		WUStationID:       pbConfig.WuStationId,
		WUAPIKey:          pbConfig.WuApiKey,
		AerisEnabled:      pbConfig.AerisEnabled,
		AerisClientID:     pbConfig.AerisClientId,
		AerisClientSecret: pbConfig.AerisClientSecret,
		PWSEnabled:        pbConfig.PwsEnabled,
		PWSStationID:      pbConfig.PwsStationId,
		PWSPassword:       pbConfig.PwsPassword,
	}

	// Register via config provider
	cachedProvider, ok := r.configProvider.(*config.CachedConfigProvider)
	if !ok {
		return "", fmt.Errorf("config provider is not cached")
	}
	
	sqliteProvider, ok := cachedProvider.GetUnderlying().(*config.SQLiteProvider)
	if !ok {
		return "", fmt.Errorf("underlying provider is not SQLite")
	}
	
	stationID, err := sqliteProvider.RegisterRemoteStation(configData)
	if err != nil {
		return "", fmt.Errorf("failed to register station: %w", err)
	}

	// Log registration
	if pbConfig.StationId == "" {
		r.logger.Infof("Generated new station ID %s for %s", stationID, pbConfig.StationName)
	} else {
		r.logger.Infof("Re-registering station %s (%s)", pbConfig.StationName, stationID)
	}

	// Build and cache station in memory
	station := &RemoteStation{
		StationID:   stationID,
		StationName: pbConfig.StationName,
		StationType: pbConfig.StationType,
		LastSeen:    time.Now(),
	}

	if pbConfig.AprsEnabled {
		station.APRS = &APRSConfig{
			Enabled:  true,
			Callsign: pbConfig.AprsCallsign,
			Password: pbConfig.AprsPassword,
		}
	}

	if pbConfig.WuEnabled {
		station.WU = &WUConfig{
			Enabled:   true,
			StationID: pbConfig.WuStationId,
			APIKey:    pbConfig.WuApiKey,
		}
	}

	if pbConfig.AerisEnabled {
		station.Aeris = &AerisConfig{
			Enabled:      true,
			ClientID:     pbConfig.AerisClientId,
			ClientSecret: pbConfig.AerisClientSecret,
		}
	}

	if pbConfig.PwsEnabled {
		station.PWS = &PWSConfig{
			Enabled:   true,
			StationID: pbConfig.PwsStationId,
			Password:  pbConfig.PwsPassword,
		}
	}

	// Update cache
	r.mu.Lock()
	r.stations[stationID] = station
	r.mu.Unlock()

	return stationID, nil
}

// GetByID retrieves a remote station by ID from cache
func (r *RemoteStationRegistry) GetByID(stationID string) *RemoteStation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stations[stationID]
}

// UpdateLastSeen updates the last seen timestamp for a station
func (r *RemoteStationRegistry) UpdateLastSeen(stationID string) {
	// Update in-memory cache
	r.mu.Lock()
	station, exists := r.stations[stationID]
	if exists {
		station.LastSeen = time.Now()
	}
	r.mu.Unlock()

	// Update database asynchronously via config provider
	if exists {
		go func() {
			cachedProvider, ok := r.configProvider.(*config.CachedConfigProvider)
			if !ok {
				return
			}
			
			sqliteProvider, ok := cachedProvider.GetUnderlying().(*config.SQLiteProvider)
			if !ok {
				return
			}
			
			if err := sqliteProvider.UpdateRemoteStationLastSeen(stationID); err != nil {
				r.logger.Debugf("failed to update last seen for station %s: %v", stationID, err)
			}
		}()
	}
}

// GetAllStations returns all registered remote stations from cache
func (r *RemoteStationRegistry) GetAllStations() []*RemoteStation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	stations := make([]*RemoteStation, 0, len(r.stations))
	for _, station := range r.stations {
		stations = append(stations, station)
	}
	return stations
}