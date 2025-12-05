// Weather Dashboard Data Service Module
// This module handles all API calls and data fetching

const WeatherDataService = (function() {
    'use strict';
    
    // API endpoints
    const endpoints = {
        latest: '/latest',
        snow: '/snow',
        stationinfo: '/stationinfo',
        span: function(hours) { return '/span/' + hours + 'h'; },
        forecast: function(hours) { return '/forecast/' + hours; }
    };
    
    // Fetch latest weather data
    const fetchLatestWeather = async (stationId) => {
        const url = stationId ? endpoints.latest + '?station_id=' + stationId : endpoints.latest;
        return WeatherUtils.fetchWithTimeout(url);
    };
    
    // Fetch snow data
    const fetchSnowData = async (stationId) => {
        const url = stationId ? endpoints.snow + '?station_id=' + stationId : endpoints.snow;
        return WeatherUtils.fetchWithTimeout(url);
    };

    // Fetch snow accumulation events for visualization
    const fetchSnowEvents = async (hours) => {
        try {
            const url = '/snow-events?hours=' + hours;
            // fetchWithTimeout returns parsed JSON directly (or null on error)
            const data = await WeatherUtils.fetchWithTimeout(url);
            if (!data) {
                console.warn('[SNOW DEBUG] Snow events fetch failed: no data returned');
                return [];
            }
            console.log('[SNOW DEBUG] Snow events received from API:', data.events?.length || 0, 'events');
            return data.events || [];
        } catch (error) {
            console.error('[SNOW DEBUG] Error in fetchSnowEvents:', error);
            return [];
        }
    };

    // Fetch historical data for charts
    const fetchHistoricalData = async (hours, station, stationId) => {
        let url = endpoints.span(hours) + '?station=' + station;
        if (stationId) {
            url += '&station_id=' + stationId;
        }
        return WeatherUtils.fetchWithTimeout(url);
    };
    
    // Fetch forecast data
    const fetchForecast = async (hours, stationId) => {
        const url = stationId ? endpoints.forecast(hours) + '?station_id=' + stationId : endpoints.forecast(hours);
        return WeatherUtils.fetchWithTimeout(url);
    };

    // Fetch station info (includes snow device ID)
    const fetchStationInfo = async () => {
        return WeatherUtils.fetchWithTimeout(endpoints.stationinfo);
    };

    // Get snow device ID from station info
    const getSnowDeviceId = (stationInfo) => {
        if (!stationInfo || !stationInfo.snow_device || !stationInfo.stations) {
            return null;
        }
        const snowDeviceName = stationInfo.snow_device;
        const snowStation = stationInfo.stations.find(s => s.name === snowDeviceName);
        return snowStation ? snowStation.id : null;
    };

    // Get air quality device ID from station info
    const getAirQualityDeviceId = (stationInfo) => {
        if (!stationInfo || !stationInfo.air_quality_device || !stationInfo.stations) {
            return null;
        }
        const aqDeviceName = stationInfo.air_quality_device;
        const aqStation = stationInfo.stations.find(s => s.name === aqDeviceName);
        return aqStation ? aqStation.id : null;
    };

    // Combined fetch for live data (weather + snow if enabled + air quality if enabled)
    const fetchLiveData = async (snowEnabled = false, airQualityEnabled = false, stationId = null, airQualityStationId = null, snowDeviceId = null) => {
        const promises = [fetchLatestWeather(stationId)];

        if (snowEnabled) {
            promises.push(fetchSnowData(snowDeviceId));
        }
        
        // Fetch air quality data from the air quality device using its station_id
        if (airQualityEnabled && airQualityStationId !== null && airQualityStationId !== undefined) {
            promises.push(fetchLatestWeather(airQualityStationId));
        }
        
        try {
            const results = await Promise.all(promises);
            let airQualityData = null;
            
            // Extract air quality data from the air quality station response
            if (airQualityEnabled && results[snowEnabled ? 2 : 1]) {
                const aqResponse = results[snowEnabled ? 2 : 1];
                airQualityData = {
                    pm25: aqResponse.pm25 !== undefined ? aqResponse.pm25 : null,
                    pm10: aqResponse.extrafloat2 !== undefined ? aqResponse.extrafloat2 : null,  // PM10 stored in extrafloat2
                    pm1: aqResponse.extrafloat1 !== undefined ? aqResponse.extrafloat1 : null,   // PM1.0 stored in extrafloat1
                    co2: aqResponse.co2 !== undefined ? aqResponse.co2 : null,
                    tvoc: aqResponse.extrafloat3 !== undefined ? aqResponse.extrafloat3 : null,  // TVOC stored in extrafloat3
                    nox: aqResponse.extrafloat4 !== undefined ? aqResponse.extrafloat4 : null    // NOx stored in extrafloat4
                };
            }
            
            return {
                weather: results[0],
                snow: results[1] || null,
                airQuality: airQualityData
            };
        } catch (error) {
            console.error('Error fetching live data:', error);
            return { weather: null, snow: null, airQuality: null };
        }
    };
    
    // Combined fetch for chart data
    const fetchChartData = async (hours, config) => {
        const { pullFromDevice, snowEnabled, snowDevice, airQualityEnabled, airQualityDevice, airQualityDeviceID, stationID } = config;
        const promises = [fetchHistoricalData(hours, pullFromDevice, stationID)];
        
        if (snowEnabled && snowDevice) {
            promises.push(fetchHistoricalData(hours, snowDevice, stationID));
        }
        
        if (airQualityEnabled && airQualityDevice && airQualityDeviceID !== null && airQualityDeviceID !== undefined) {
            promises.push(fetchHistoricalData(hours, airQualityDevice, airQualityDeviceID));
        }
        
        try {
            const results = await Promise.all(promises);
            return {
                mainData: results[0],
                snowData: snowEnabled ? (results[1] || null) : null,
                airQualityData: airQualityEnabled ? (results[snowEnabled ? 2 : 1] || null) : null
            };
        } catch (error) {
            console.error('Error fetching chart data:', error);
            return { mainData: null, snowData: null, airQualityData: null };
        }
    };
    
    // Process live weather data into display format
    const processLiveWeatherData = (data) => {
        if (!data) return null;
        
        return {
            // Temperature & Atmospheric
            temperature: WeatherUtils.formatTemperatureValue(data.otemp),
            feelsLike: WeatherUtils.formatTemperatureValue(
                data.heatidx || data.windch || data.otemp
            ),
            humidity: WeatherUtils.formatValue(data.ohum, 1),
            dewPoint: WeatherUtils.formatTemperatureValue(
                WeatherUtils.calculateDewPoint(data.otemp, data.ohum)
            ),
            barometer: WeatherUtils.formatValue(data.bar, 2),
            solar: WeatherUtils.formatValue(data.solarwatts, 1),
            skyConditions: WeatherUtils.calculateSkyConditions(
                data.solarwatts, 
                data.potentialsolarwatts
            ),
            
            // Wind
            windSpeed: data.winds !== null && data.winds !== undefined ? Math.round(parseFloat(data.winds)) : '--',
            windGusts: data.windgust !== null && data.windgust !== undefined ? Math.round(parseFloat(data.windgust)) : '--',
            windDirection: data.windcard || '--',
            windDegrees: data.windd || '--',
            
            // Precipitation
            rainToday: WeatherUtils.formatValue(data.rainday, 2),
            rainRate: WeatherUtils.formatValue(data.rainrate, 2),
            rain24hr: WeatherUtils.formatValue(data.rainfall24h, 2),
            rain48hr: WeatherUtils.formatValue(data.rainfall48h, 2),
            rain72hr: WeatherUtils.formatValue(data.rainfall72h, 2),
            rainStormTotal: WeatherUtils.formatValue(data.rainfallstorm, 2),
            
            // Battery
            batteryVoltage: data.stationbatteryvoltage,
            
            // Raw data for other uses
            raw: data
        };
    };
    
    // Process snow data into display format
    const processSnowData = (data) => {
        if (!data) {
            return {
                depth: '0.00',
                last24: '0.00',
                last72: '0.00',
                stormTotal: '0.00',
                seasonTotal: '0.00',
                accumulationRate: '0.00'
            };
        }

        let accumulationRate = '0.00';
        if (data.snowfallrate != null && data.snowfallrate > 0) {
            accumulationRate = parseFloat(data.snowfallrate).toFixed(3);
        } else if (data.snowlast24 && data.snowlast24 > 0) {
            accumulationRate = (data.snowlast24 / 24).toFixed(3);
        }

        return {
            depth: WeatherUtils.formatValue(data.snowdepth, 2),
            last24: WeatherUtils.formatValue(data.snowlast24, 2),
            last72: WeatherUtils.formatValue(data.snowlast72, 2),
            stormTotal: WeatherUtils.formatValue(data.snowstorm, 2),
            seasonTotal: WeatherUtils.formatValue(data.snowseason, 2),
            accumulationRate: accumulationRate
        };
    };
    
    // Process forecast data
    const processForecastData = (data, type) => {
        if (!data || !data.data) return null;
        
        if (type === 'week') {
            return data.data.slice(0, 10).map((interval, i) => {
                const date = new Date(interval.dateTimeISO);
                const isSnow = WeatherUtils.isSnowWeather(interval.weatherPrimaryCoded);
                const isRain = WeatherUtils.isRainWeather(interval.weatherPrimaryCoded);
                
                let dayName;
                if (i === 0) {
                    dayName = "Today";
                } else if (i === 1) {
                    dayName = "Tomorrow";
                } else if (i >= 7) {
                    // For days 7+, add "Next" prefix
                    dayName = "Next " + WeatherUtils.getDayName(date.getDay());
                } else {
                    dayName = WeatherUtils.getDayName(date.getDay());
                }
                
                return {
                    dayName: dayName,
                    highTemp: interval.maxTempF,
                    lowTemp: interval.minTempF,
                    icon: interval.weatherIcon,
                    weather: interval.compactWeather,
                    precipType: isSnow ? 'snow' : (isRain ? 'rain' : 'none'),
                    precipIcon: isSnow ? '❄' : (isRain ? '⛆' : ''),
                    precipAmount: isSnow ? interval.snowIN + '"' : 
                                 (isRain ? interval.precipIN + '"' : '')
                };
            });
        } else if (type === 'day') {
            return data.data.slice(0, 24).map((interval, i) => {
                const date = moment(interval.dateTimeISO, "YYYY-MM-DD HH:mm:ss.SSSSSS-ZZ");
                const isSnow = WeatherUtils.isSnowWeather(interval.weatherPrimaryCoded);
                const isRain = WeatherUtils.isRainWeather(interval.weatherPrimaryCoded);
                
                return {
                    time: (date.hour() === 0 || i === 0) 
                        ? date.format("h A") + '<br>' + date.format("ddd") 
                        : date.format("h A"),
                    temp: interval.avgTempF,
                    icon: interval.weatherIcon,
                    precipType: isSnow ? 'snow' : (isRain ? 'rain' : 'none'),
                    precipIcon: isSnow ? '❄' : (isRain ? '⛆' : ''),
                    precipAmount: isSnow ? interval.snowIN + '"' : 
                                 (isRain ? interval.precipIN + '"' : interval.pop + '%')
                };
            });
        }
        
        return null;
    };
    
    // Public API
    return {
        // Raw fetch methods
        fetchLatestWeather,
        fetchSnowData,
        fetchSnowEvents,
        fetchHistoricalData,
        fetchForecast,
        fetchStationInfo,

        // Helper methods
        getSnowDeviceId,
        getAirQualityDeviceId,

        // Combined fetch methods
        fetchLiveData,
        fetchChartData,

        // Data processing methods
        processLiveWeatherData,
        processSnowData,
        processForecastData
    };
})();

// Make available globally if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherDataService;
}