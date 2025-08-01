// Weather Dashboard Data Service Module
// This module handles all API calls and data fetching

const WeatherDataService = (function() {
    'use strict';
    
    // API endpoints
    const endpoints = {
        latest: '/latest',
        snow: '/snow',
        span: (hours) => `/span/${hours}h`,
        forecast: (hours) => `/forecast/${hours}`
    };
    
    // Fetch latest weather data
    const fetchLatestWeather = async () => {
        return WeatherUtils.fetchWithTimeout(endpoints.latest);
    };
    
    // Fetch snow data
    const fetchSnowData = async () => {
        return WeatherUtils.fetchWithTimeout(endpoints.snow);
    };
    
    // Fetch historical data for charts
    const fetchHistoricalData = async (hours, station) => {
        const url = `${endpoints.span(hours)}?station=${station}`;
        return WeatherUtils.fetchWithTimeout(url);
    };
    
    // Fetch forecast data
    const fetchForecast = async (hours) => {
        return WeatherUtils.fetchWithTimeout(endpoints.forecast(hours));
    };
    
    // Combined fetch for live data (weather + snow if enabled)
    const fetchLiveData = async (snowEnabled = false) => {
        const promises = [fetchLatestWeather()];
        
        if (snowEnabled) {
            promises.push(fetchSnowData());
        }
        
        try {
            const results = await Promise.all(promises);
            return {
                weather: results[0],
                snow: results[1] || null
            };
        } catch (error) {
            console.error('Error fetching live data:', error);
            return { weather: null, snow: null };
        }
    };
    
    // Combined fetch for chart data
    const fetchChartData = async (hours, config) => {
        const { pullFromDevice, snowEnabled, snowDevice } = config;
        const promises = [fetchHistoricalData(hours, pullFromDevice)];
        
        if (snowEnabled && snowDevice) {
            promises.push(fetchHistoricalData(hours, snowDevice));
        }
        
        try {
            const results = await Promise.all(promises);
            return {
                mainData: results[0],
                snowData: results[1] || null
            };
        } catch (error) {
            console.error('Error fetching chart data:', error);
            return { mainData: null, snowData: null };
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
            rainToday: WeatherUtils.formatValue(data.dayrain, 2),
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
                    precipAmount: isSnow ? `${interval.snowIN}"` : 
                                 (isRain ? `${interval.precipIN}"` : '')
                };
            });
        } else if (type === 'day') {
            return data.data.slice(0, 24).map((interval, i) => {
                const date = moment(interval.dateTimeISO, "YYYY-MM-DD HH:mm:ss.SSSSSS-ZZ");
                const isSnow = WeatherUtils.isSnowWeather(interval.weatherPrimaryCoded);
                const isRain = WeatherUtils.isRainWeather(interval.weatherPrimaryCoded);
                
                return {
                    time: (date.hour() === 0 || i === 0) 
                        ? `${date.format("h A")}<br>${date.format("ddd")}` 
                        : date.format("h A"),
                    temp: interval.avgTempF,
                    icon: interval.weatherIcon,
                    precipType: isSnow ? 'snow' : (isRain ? 'rain' : 'none'),
                    precipIcon: isSnow ? '❄' : (isRain ? '⛆' : ''),
                    precipAmount: isSnow ? `${interval.snowIN}"` : 
                                 (isRain ? `${interval.precipIN}"` : `${interval.pop}%`)
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
        fetchHistoricalData,
        fetchForecast,
        
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