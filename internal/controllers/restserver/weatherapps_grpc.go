package restserver

import (
	"context"
	"fmt"
	"time"

	"github.com/chrissnell/remoteweather/internal/controllers"
	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/grpcutil"
	"github.com/chrissnell/remoteweather/internal/log"
	weatherapps "github.com/chrissnell/remoteweather/protocols/weatherapps"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetWeatherTimeSpan handles weatherapps gRPC requests for weather data over a time span
func (c *Controller) GetWeatherTimeSpan(ctx context.Context, request *weatherapps.WeatherSpanRequest) (*weatherapps.WeatherSpan, error) {
	// Validate station name
	if err := grpcutil.ValidateStationRequest(request.StationName, c.DeviceManager); err != nil {
		return nil, err
	}

	// Get snow base distance for snow depth calculations
	baseDistance := c.getSnowBaseDistanceForStation(request.StationName)

	// Use shared database fetching logic
	span := request.SpanDuration.AsDuration()
	dbFetchedReadings, err := c.fetchWeatherSpan(request.StationName, span, baseDistance)
	if err != nil {
		return nil, err
	}

	// Transform readings to weatherapps protobuf format
	readings := make([]*weatherapps.WeatherReading, 0, len(dbFetchedReadings))
	for i := range dbFetchedReadings {
		reading := transformBucketReadingToWeatherApps(&dbFetchedReadings[i])
		readings = append(readings, reading)
	}

	// Note: For span readings, we don't calculate per-reading rain totals
	// as they would be expensive and not particularly useful for historical data

	spanResponse := &weatherapps.WeatherSpan{
		SpanStart: timestamppb.New(time.Now().Add(-span)),
		Reading:   readings,
	}

	return spanResponse, nil
}

// GetCurrentReading handles weatherapps gRPC requests for the latest weather reading
// This includes calculated values like rain totals, wind gust, etc.
func (c *Controller) GetCurrentReading(ctx context.Context, request *weatherapps.LatestReadingRequest) (*weatherapps.WeatherReading, error) {
	// Validate station name
	if err := grpcutil.ValidateStationRequest(request.StationName, c.DeviceManager); err != nil {
		return nil, err
	}

	// Get snow base distance for snow depth calculations
	baseDistance := c.getSnowBaseDistanceForStation(request.StationName)

	// Use shared database fetching logic
	latestReading, err := c.fetchLatestReading(request.StationName, baseDistance)
	if err != nil {
		return nil, err
	}

	// Transform reading to weatherapps protobuf format
	reading := transformBucketReadingToWeatherApps(latestReading)

	// Add calculated rainfall totals (matching /latest REST endpoint behavior)
	if err := c.addCalculatedRainTotals(reading, request.StationName); err != nil {
		log.Warnf("Failed to calculate rain totals for station %s: %v", request.StationName, err)
		// Continue anyway - we have the base reading
	}

	// Add calculated wind gust (matching /latest REST endpoint behavior)
	if err := c.addCalculatedWindGust(reading, request.StationName); err != nil {
		log.Warnf("Failed to calculate wind gust for station %s: %v", request.StationName, err)
		// Continue anyway - we have the base reading
	}

	// Add calculated rain rate (matching /latest REST endpoint behavior)
	if err := c.addCalculatedRainRate(reading, request.StationName); err != nil {
		log.Warnf("Failed to calculate rain rate for station %s: %v", request.StationName, err)
		// Continue anyway - we have the base reading
	}

	return reading, nil
}

// StreamLiveWeather streams live weather data using the weatherapps protocol
// Polls the database every 3 seconds and includes all calculated values
func (c *Controller) StreamLiveWeather(req *weatherapps.LiveWeatherRequest, stream weatherapps.WeatherAppsV1_StreamLiveWeatherServer) error {
	ctx := stream.Context()
	p, _ := peer.FromContext(ctx)

	if !c.DBEnabled {
		return fmt.Errorf("database not configured")
	}

	// Validate station name
	if err := grpcutil.ValidateStationRequest(req.StationName, c.DeviceManager); err != nil {
		return err
	}

	log.Infof("Starting live weatherapps stream for client [%v] requesting station [%s]", p.Addr, req.StationName)

	// Get snow base distance for snow depth calculations
	baseDistance := c.getSnowBaseDistanceForStation(req.StationName)

	// Poll database every 3 seconds
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	var lastReadingTime time.Time

	// Send initial reading immediately
	if bucketReading, err := c.fetchLatestReading(req.StationName, baseDistance); err == nil {
		reading := transformBucketReadingToWeatherApps(bucketReading)
		c.addCalculatedRainTotals(reading, req.StationName)
		c.addCalculatedWindGust(reading, req.StationName)
		c.addCalculatedRainRate(reading, req.StationName)

		if err := stream.Send(reading); err != nil {
			return err
		}
		lastReadingTime = bucketReading.Bucket
		log.Debugf("Sent initial weatherapps reading to client [%v] for station [%s]", p.Addr, req.StationName)
	}

	for {
		select {
		case <-ctx.Done():
			log.Infof("Client [%v] disconnected from live weatherapps stream for station [%s]", p.Addr, req.StationName)
			return nil
		case <-ticker.C:
			// Query latest reading from database
			bucketReading, err := c.fetchLatestReading(req.StationName, baseDistance)
			if err != nil {
				// Log but don't disconnect client - station might be temporarily offline
				log.Debugf("No reading found for station %s: %v", req.StationName, err)
				continue
			}

			// Only send if this is a new reading
			if bucketReading.Bucket.After(lastReadingTime) {
				reading := transformBucketReadingToWeatherApps(bucketReading)
				c.addCalculatedRainTotals(reading, req.StationName)
				c.addCalculatedWindGust(reading, req.StationName)
				c.addCalculatedRainRate(reading, req.StationName)

				if err := stream.Send(reading); err != nil {
					log.Errorf("Error sending weatherapps reading to client [%v]: %v", p.Addr, err)
					return err
				}
				lastReadingTime = bucketReading.Bucket
				log.Debugf("Sent new weatherapps reading to client [%v] for station [%s]", p.Addr, req.StationName)
			}
		}
	}
}

// Helper functions for adding calculated values

// addCalculatedRainTotals adds rain24h, rain48h, rain72h, and rainStormTotal to the reading
// Uses the same database queries as the /latest REST endpoint
func (c *Controller) addCalculatedRainTotals(reading *weatherapps.WeatherReading, stationName string) error {
	// Use the optimized function that combines summary + recent rain
	type RainfallPeriods struct {
		Rain24h float32 `gorm:"column:rain_24h"`
		Rain48h float32 `gorm:"column:rain_48h"`
		Rain72h float32 `gorm:"column:rain_72h"`
	}
	var rainfallPeriods RainfallPeriods

	// This query uses the pre-calculated summary and adds recent rain since last update
	err := c.DB.Raw(`
		SELECT * FROM get_rainfall_with_recent(?)
	`, stationName).Scan(&rainfallPeriods).Error

	if err != nil {
		// Fallback to direct calculation if summary is not available
		log.Warnf("Failed to get rainfall from summary, falling back to direct calculation: %v", err)
		c.DB.Raw(`
			SELECT
				COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '24 hours' THEN period_rain END), 0) as rain_24h,
				COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '48 hours' THEN period_rain END), 0) as rain_48h,
				COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '72 hours' THEN period_rain END), 0) as rain_72h
			FROM weather_5m
			WHERE stationname = ? AND bucket >= NOW() - INTERVAL '72 hours'
		`, stationName).Scan(&rainfallPeriods)
	}

	reading.Rain24H = rainfallPeriods.Rain24h
	reading.Rain48H = rainfallPeriods.Rain48h
	reading.Rain72H = rainfallPeriods.Rain72h

	// Storm rainfall total using existing function
	type StormRainResult struct {
		StormStart    *time.Time `gorm:"column:storm_start"`
		StormEnd      *time.Time `gorm:"column:storm_end"`
		TotalRainfall float32    `gorm:"column:total_rainfall"`
	}
	var stormResult StormRainResult
	err = c.DB.Raw("SELECT * FROM calculate_storm_rainfall(?) LIMIT 1", stationName).Scan(&stormResult).Error
	if err != nil {
		log.Errorf("error getting storm rainfall from DB: %v", err)
		return err
	}
	reading.RainStormTotal = stormResult.TotalRainfall

	return nil
}

// addCalculatedWindGust adds the 10-minute wind gust to the reading
func (c *Controller) addCalculatedWindGust(reading *weatherapps.WeatherReading, stationName string) error {
	type WindGustResult struct {
		WindGust float32
	}
	var windGustResult WindGustResult
	query := "SELECT calculate_wind_gust(?) AS wind_gust"
	err := c.DB.Raw(query, stationName).Scan(&windGustResult).Error
	if err != nil {
		log.Errorf("error getting wind gust from DB: %v", err)
		return err
	}
	reading.WindGust = windGustResult.WindGust
	return nil
}

// addCalculatedRainRate adds the calculated rain rate to the reading
func (c *Controller) addCalculatedRainRate(reading *weatherapps.WeatherReading, stationName string) error {
	dbClient := &database.Client{DB: c.DB}
	rainRate := controllers.CalculateRainRate(dbClient, stationName)
	reading.RainRate = rainRate
	return nil
}
