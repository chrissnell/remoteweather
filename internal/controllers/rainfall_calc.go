package controllers

import (
	"time"

	"github.com/chrissnell/remoteweather/internal/database"
	"github.com/chrissnell/remoteweather/internal/log"
)

// CalculateDailyRainfall calculates the total rainfall for the current day (since midnight)
// from incremental measurements. This uses the same optimized two-query approach as the
// REST server (commit 45a3e98) to avoid slow subqueries.
func CalculateDailyRainfall(db *database.Client, stationName string) float32 {
	// Get today's rainfall in two fast queries to avoid slow subquery
	var aggregatedRain float32
	var lastBucket *time.Time
	
	// First get aggregated rain and find last bucket time
	var result struct {
		Total      float32    `gorm:"column:total"`
		LastBucket *time.Time `gorm:"column:last_bucket"`
	}
	db.DB.Raw(`
		SELECT 
			COALESCE(SUM(period_rain), 0) as total,
			MAX(bucket) as last_bucket
		FROM weather_5m 
		WHERE stationname = ? 
		AND bucket >= date_trunc('day', NOW())
	`, stationName).Scan(&result)
	aggregatedRain = result.Total
	lastBucket = result.LastBucket
	
	// Then get incremental rain since last bucket (if any)
	var incrementalRain float32
	if lastBucket != nil {
		db.DB.Raw(`
			SELECT COALESCE(SUM(rainincremental), 0) as total
			FROM weather
			WHERE stationname = ?
			AND time > ?
			AND time >= NOW() - INTERVAL '1 hour'
		`, stationName, lastBucket).Scan(&incrementalRain)
	}
	
	totalRainfall := aggregatedRain + incrementalRain
	
	log.Debugf("Daily rainfall for %s: %.2f (aggregated: %.2f, incremental: %.2f)",
		stationName, totalRainfall, aggregatedRain, incrementalRain)

	return totalRainfall
}

// CalculateRainRate calculates the current rain rate in inches per hour
// by looking at rain incremental values over the last 10 minutes and extrapolating to an hourly rate.
func CalculateRainRate(db *database.Client, stationName string) float32 {
	// Get the sum of rainincremental over the last 10 minutes
	var rainLast10Min float32

	db.DB.Raw(`
		SELECT COALESCE(SUM(rainincremental), 0) as total
		FROM weather
		WHERE stationname = ?
		AND time >= NOW() - INTERVAL '10 minutes'
	`, stationName).Scan(&rainLast10Min)

	// Extrapolate to hourly rate: rain in 10 minutes * 6 = rain per hour
	rainRate := rainLast10Min * 6.0

	log.Debugf("Rain rate for %s: %.2f in/hr (10-min total: %.2f in)",
		stationName, rainRate, rainLast10Min)

	return rainRate
}