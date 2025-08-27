// Weather Portal Utility Functions
// Pure utility functions for formatting, calculations, and data processing

const PortalUtils = {
    // Temperature formatting and color functions
    formatTemperature(temp) {
        return temp ? `${Math.round(parseFloat(temp))}°F` : '--';
    },

    formatHumidity(humidity) {
        return humidity ? `${Math.round(parseFloat(humidity))}%` : '--';
    },

    formatPressure(pressure) {
        return pressure ? `${parseFloat(pressure).toFixed(2)} inHg` : '--';
    },

    formatWindSpeed(speed) {
        return speed ? `${Math.round(parseFloat(speed))} mph` : '--';
    },

    // Wind direction calculations
    getWindDirection(weatherData) {
        if (weatherData.windd) {
            return parseFloat(weatherData.windd);
        }
        return null;
    },

    getWindDirectionText(weatherData) {
        const dir = this.getWindDirection(weatherData);
        if (dir === null) return '--';
        
        const directions = ['N', 'NNE', 'NE', 'ENE', 'E', 'ESE', 'SE', 'SSE', 
                          'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW'];
        const index = Math.round(dir / 22.5) % 16;
        return directions[index];
    },

    // Data value extraction
    getDataValue(station, dataType) {
        if (!station.weather) return null;
        
        switch (dataType) {
            case 'temperature':
                return station.weather.otemp ? parseFloat(station.weather.otemp) : null;
            case 'humidity':
                return station.weather.ohum ? parseFloat(station.weather.ohum) : null;
            case 'rainfall':
                return station.weather.dayrain ? parseFloat(station.weather.dayrain) : null;
            case 'snow':
                return station.weather.snowdepth ? parseFloat(station.weather.snowdepth) : null;
            case 'barometer':
                return station.weather.bar ? parseFloat(station.weather.bar) : null;
            case 'wind':
                return station.weather.winds ? parseFloat(station.weather.winds) : null;
            case 'airquality':
                // Return the higher of PM2.5 or PM10 AQI values
                const pm25 = station.weather.aqi_pm25_aqin ? parseFloat(station.weather.aqi_pm25_aqin) : null;
                const pm10 = station.weather.aqi_pm10_aqin ? parseFloat(station.weather.aqi_pm10_aqin) : null;
                if (pm25 === null && pm10 === null) return null;
                if (pm25 === null) return pm10;
                if (pm10 === null) return pm25;
                return Math.max(pm25, pm10);
            default:
                return null;
        }
    },

    formatDataValue(value, dataType) {
        if (value === null || value === undefined) return '--';
        
        switch (dataType) {
            case 'temperature':
                return `${Math.round(value)}`;
            case 'humidity':
                return `${Math.round(value)}%`;
            case 'rainfall':
                return `${value.toFixed(1)}"`;
            case 'snow':
                return `${value.toFixed(1)}"`;
            case 'barometer':
                return `${value.toFixed(1)}`;
            case 'wind':
                return `${Math.round(value)}`;
            case 'airquality':
                return `${Math.round(value)}`;
            default:
                return '--';
        }
    },

    // Color calculation functions
    getDataColor(value, dataType) {
        if (value === null || value === undefined) return '#7f8c8d'; // Gray for no data
        
        switch (dataType) {
            case 'temperature':
                return this.getTemperatureColor(value);
            case 'humidity':
                return this.getHumidityColor(value);
            case 'rainfall':
                return this.getRainfallColor(value);
            case 'snow':
                return this.getSnowColor(value);
            case 'barometer':
                return this.getBarometerColor(value);
            case 'wind':
                return this.getWindColor(value);
            case 'airquality':
                return this.getAirQualityColor(value);
            default:
                return '#3498db'; // Default blue
        }
    },

    getTemperatureColor(temp) {
        if (temp >= 90) return '#8b0000'; // Dark red
        if (temp >= 80) return '#b22222'; // Fire brick
        if (temp >= 70) return '#cd5c5c'; // Indian red
        if (temp >= 60) return '#ffa500'; // Orange
        if (temp >= 50) return '#3498db'; // Blue
        if (temp >= 40) return '#2980b9'; // Darker blue
        if (temp >= 30) return '#1f4e79'; // Dark blue
        if (temp >= 20) return '#0f2e50'; // Very dark blue
        return '#4b0082'; // Indigo (very cold)
    },

    getHumidityColor(humidity) {
        if (humidity >= 80) return '#8b0000'; // Dark red
        if (humidity >= 70) return '#b22222'; // Fire brick
        if (humidity >= 60) return '#cd5c5c'; // Indian red
        if (humidity >= 50) return '#ffa500'; // Orange
        if (humidity >= 40) return '#3498db'; // Blue
        return '#2980b9'; // Darker blue for low humidity
    },

    getRainfallColor(rainfall) {
        if (rainfall >= 2.0) return '#8b0000'; // Dark red
        if (rainfall >= 1.5) return '#b22222'; // Fire brick
        if (rainfall >= 1.0) return '#cd5c5c'; // Indian red
        if (rainfall >= 0.5) return '#ffa500'; // Orange
        if (rainfall >= 0.1) return '#3498db'; // Blue
        return '#2980b9'; // Darker blue for no rain
    },

    getSnowColor(snowDepth) {
        if (snowDepth >= 24) return '#8b0000'; // Dark red
        if (snowDepth >= 18) return '#b22222'; // Fire brick
        if (snowDepth >= 12) return '#cd5c5c'; // Indian red
        if (snowDepth >= 6) return '#ffa500'; // Orange
        if (snowDepth >= 1) return '#3498db'; // Blue
        return '#2980b9'; // Darker blue for no snow
    },

    getBarometerColor(pressure) {
        // Lower pressure = darker red (storm conditions)
        // Normal pressure is around 29.92 inHg
        if (pressure <= 29.50) return '#8b0000'; // Dark red (very low)
        if (pressure <= 29.70) return '#b22222'; // Fire brick (low)
        if (pressure <= 29.85) return '#cd5c5c'; // Indian red (below normal)
        if (pressure <= 30.00) return '#3498db'; // Blue (normal)
        if (pressure <= 30.15) return '#2980b9'; // Darker blue (above normal)
        return '#1f4e79'; // Dark blue (high pressure)
    },

    getWindColor(windSpeed) {
        if (windSpeed >= 40) return '#8b0000'; // Dark red (very high)
        if (windSpeed >= 30) return '#b22222'; // Fire brick (high)
        if (windSpeed >= 20) return '#cd5c5c'; // Indian red (moderate-high)
        if (windSpeed >= 15) return '#ffa500'; // Orange (moderate)
        if (windSpeed >= 10) return '#3498db'; // Blue (light)
        if (windSpeed >= 5) return '#2980b9'; // Darker blue (very light)
        return '#1f4e79'; // Dark blue (calm)
    },

    getWindTriangleColor(windSpeed) {
        // Return contrasting triangle color based on wind speed circle color
        if (windSpeed >= 40) return '#00ff7f'; // Bright green (contrasts with dark red)
        if (windSpeed >= 30) return '#00bfff'; // Deep sky blue (contrasts with fire brick)
        if (windSpeed >= 20) return '#ffffff'; // White (contrasts with indian red)
        if (windSpeed >= 15) return '#1f4e79'; // Dark blue (contrasts with orange)
        if (windSpeed >= 10) return '#ff6b35'; // Orange (contrasts with blue)
        if (windSpeed >= 5) return '#ff6b35'; // Orange (contrasts with darker blue)
        return '#ff6b35'; // Orange (contrasts with dark blue - calm)
    },

    getAirQualityColor(aqi) {
        // Solarized color scheme for AQI
        if (aqi <= 50) return '#859900'; // Solarized green - Good
        if (aqi <= 100) return '#b58900'; // Solarized yellow - Moderate  
        if (aqi <= 150) return '#cb4b16'; // Solarized orange - Unhealthy for Sensitive Groups
        if (aqi <= 200) return '#dc322f'; // Solarized red - Unhealthy
        if (aqi <= 300) return '#d33682'; // Solarized magenta - Very Unhealthy
        return '#6c71c4'; // Solarized violet - Hazardous
    },

    // Check if station is an air quality station based on device type
    isAirQualityStation(station) {
        if (!station) return false;
        
        // Check the station type - currently only 'airgradient' is an air quality station
        return station.type === 'airgradient';
    },

    // Station status functions
    getStatusText(station) {
        if (station.status === 'offline') return 'Offline';
        if (station.status === 'error') return 'Error';
        
        if (station.lastUpdate) {
            const ageMinutes = (new Date() - station.lastUpdate) / (1000 * 60);
            if (ageMinutes > 60) return 'Stale data';
            return 'Online';
        }
        
        return 'Unknown';
    },

    getMarkerClass(station, currentDisplayType) {
        if (station.status === 'offline') return 'offline';
        if (station.status === 'error') return 'error';
        
        // Check if the specific data type is available
        const dataValue = this.getDataValue(station, currentDisplayType);
        if (dataValue === null || dataValue === undefined) {
            return 'no-data';
        }
        
        // Check for data freshness (warning if older than 1 hour)
        if (station.weather && station.lastUpdate) {
            const ageMinutes = (new Date() - station.lastUpdate) / (1000 * 60);
            if (ageMinutes > 60) return 'warning';
        }
        
        return 'online';
    },

    // URL construction helper
    constructStationUrl(website) {
        if (!website || !website.hostname) return null;
        
        // Construct URL with proper protocol and port
        let url = `${website.protocol}://${website.hostname}`;
        
        // Only include port if it's not the standard port for the protocol
        const isStandardPort = (website.protocol === 'http' && website.port === 80) ||
                               (website.protocol === 'https' && website.port === 443);
        
        if (!isStandardPort) {
            url += `:${website.port}`;
        }
        
        return url;
    }
};

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = PortalUtils;
}