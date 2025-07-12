// Weather Dashboard DOM Module
// This module handles all DOM updates and element management

const WeatherDOM = (function() {
    'use strict';
    
    // DOM element cache for performance
    const elementCache = new Map();
    
    // Get cached element by ID
    const getCachedElement = (elementId) => {
        if (!elementCache.has(elementId)) {
            const element = document.getElementById(elementId);
            if (element) {
                elementCache.set(elementId, element);
            }
        }
        return elementCache.get(elementId);
    };
    
    // Update element content (text or HTML)
    const updateElement = (id, value) => {
        const element = getCachedElement(id);
        if (element) {
            // Use innerHTML if value contains HTML tags, otherwise use textContent
            if (typeof value === 'string' && value.includes('<')) {
                element.innerHTML = value;
            } else {
                element.textContent = value;
            }
        }
    };
    
    // Update multiple elements at once
    const updateElements = (updates) => {
        Object.entries(updates).forEach(([id, value]) => {
            updateElement(id, value);
        });
    };
    
    // Update live weather display
    const updateLiveWeather = (weatherData, snowData = null) => {
        if (!weatherData) return;
        
        // Temperature & Atmospheric
        updateElements({
            'temperature': weatherData.temperature,
            'feels-like': weatherData.feelsLike,
            'humidity': weatherData.humidity,
            'dew-point': weatherData.dewPoint,
            'barometer': weatherData.barometer,
            'solar': weatherData.solar,
            'sky-conditions': weatherData.skyConditions
        });
        
        // Wind
        updateElements({
            'wind-speed': weatherData.windSpeed,
            'wind-gusts': weatherData.windGusts,
            'wind-direction': weatherData.windDirection,
            'wind-degrees': `(${weatherData.windDegrees}°)`
        });
        
        // Precipitation
        updateElements({
            'rain-today': weatherData.rainToday,
            'rain-rate': weatherData.rainRate,
            'rain-24hr': weatherData.rain24hr,
            'rain-48hr': weatherData.rain48hr,
            'rain-72hr': weatherData.rain72hr,
            'rain-storm-total': weatherData.rainStormTotal
        });
        
        // Update windrose
        updateWindrose(weatherData.windDegrees, weatherData.windSpeed, weatherData.windDirection);
        
        // Update battery info
        updateBatteryInfo(weatherData.batteryVoltage);
        
        // Update snow data if provided
        if (snowData) {
            updateSnowData(snowData);
        }
    };
    
    // Update snow data display
    const updateSnowData = (snowData) => {
        updateElements({
            'snow-depth': snowData.depth,
            'snow-24hr': snowData.last24,
            'snow-72hr': snowData.last72,
            'snow-storm-total': snowData.stormTotal,
            'snow-accumulation-rate': snowData.accumulationRate
        });
    };
    
    // Update windrose display
    const updateWindrose = (direction, speed, cardinalDir) => {
        const windDirElement = getCachedElement('rdg-winddir');
        if (windDirElement && direction != null) {
            windDirElement.style.transform = `rotate(${direction}deg)`;
        }
        
        updateElements({
            'rdg-winddir-cardinal': cardinalDir || '---',
            'rdg-windspeed': speed !== null && speed !== undefined ? speed : '--'
        });
    };
    
    // Update battery info display
    const updateBatteryInfo = (voltage) => {
        if (!voltage || parseFloat(voltage) === 0) {
            updateElements({
                'battery-voltage': '--',
                'battery-status': '--'
            });
        } else {
            const v = parseFloat(voltage);
            updateElements({
                'battery-voltage': v.toFixed(2),
                'battery-status': WeatherUtils.getBatteryStatus(voltage)
            });
        }
    };
    
    // Update live indicator
    const updateLiveIndicator = (isLive = true, secondsSinceUpdate = 0) => {
        const indicatorDot = getCachedElement('live-indicator-dot');
        const updateText = getCachedElement('last-update');
        
        if (indicatorDot) {
            if (isLive) {
                indicatorDot.style.display = 'flex';
                indicatorDot.style.animation = 'livePulse 2s infinite';
                indicatorDot.style.opacity = '1';
                indicatorDot.style.backgroundColor = '';
            } else {
                indicatorDot.style.animation = 'none';
                indicatorDot.style.opacity = '0.3';
                indicatorDot.style.backgroundColor = '#666';
            }
        }
        
        if (updateText) {
            if (isLive) {
                updateText.textContent = `Last updated ${secondsSinceUpdate}s ago`;
            } else {
                updateText.textContent = 'Update failed';
            }
        }
    };
    
    // Update week forecast display
    const updateWeekForecast = (forecastData, lastUpdated) => {
        if (!forecastData) return;
        
        forecastData.forEach((day, i) => {
            updateElements({
                [`week-forecast-interval-${i}-title`]: day.dayName,
                [`week-forecast-interval-${i}-high-temp`]: day.highTemp,
                [`week-forecast-interval-${i}-low-temp`]: day.lowTemp,
                [`week-forecast-interval-${i}-icon`]: day.icon,
                [`week-forecast-interval-${i}-weather`]: day.weather,
                [`week-forecast-interval-${i}-precip-icon`]: day.precipIcon,
                [`week-forecast-interval-${i}-precip`]: day.precipAmount
            });
        });
        
        if (lastUpdated) {
            updateElement('forecast-week-last-updated', `Last Updated: ${lastUpdated}`);
        }
    };
    
    // Update day forecast display
    const updateDayForecast = (forecastData, lastUpdated, temperatureScaling) => {
        if (!forecastData) return;
        
        forecastData.forEach((hour, i) => {
            updateElements({
                [`day-forecast-interval-${i}-title`]: hour.time,
                [`day-forecast-interval-${i}-avg-temp`]: `${hour.temp}°`,
                [`day-forecast-interval-${i}-icon`]: hour.icon,
                [`day-forecast-interval-${i}-precip-icon`]: hour.precipIcon,
                [`day-forecast-interval-${i}-precip`]: hour.precipAmount
            });
            
            // Apply temperature-based positioning if scaling provided
            if (temperatureScaling) {
                const adjustableDiv = getCachedElement(`day-forecast-interval-${i}-adjustable-div`);
                if (adjustableDiv) {
                    const { pixelsPerDegree, highTemp } = temperatureScaling;
                    const paddingValue = pixelsPerDegree * (highTemp - hour.temp);
                    adjustableDiv.style.paddingTop = `${paddingValue}px`;
                }
            }
        });
        
        if (lastUpdated) {
            updateElement('forecast-day-last-updated', `Last Updated: ${lastUpdated}`);
        }
    };
    
    // Show/hide chart range containers
    const switchChartRange = (range) => {
        // Update active tab
        document.querySelectorAll('.chart-tab').forEach(tab => {
            tab.classList.remove('active');
        });
        const activeTab = document.querySelector(`[data-range="${range}"]`);
        if (activeTab) {
            activeTab.classList.add('active');
        }
        
        // Hide all chart containers
        document.querySelectorAll('.chart-range-container').forEach(container => {
            container.style.display = 'none';
        });
        
        // Show selected range container
        const targetContainer = getCachedElement(`charts-${range}`);
        if (targetContainer) {
            targetContainer.style.display = 'block';
        }
    };
    
    // Show/hide forecast type containers
    const switchForecastType = (type) => {
        // Update active tab
        document.querySelectorAll('.forecast-tab').forEach(tab => {
            tab.classList.remove('active');
        });
        const activeTab = document.querySelector(`[data-forecast-type="${type}"]`);
        if (activeTab) {
            activeTab.classList.add('active');
        }
        
        // Hide all forecast containers
        document.querySelectorAll('.forecast-container').forEach(container => {
            container.style.display = 'none';
        });
        
        // Show selected forecast container
        const targetContainer = getCachedElement(`forecast-${type}`);
        if (targetContainer) {
            targetContainer.style.display = 'block';
        }
    };
    
    // Clear element cache (useful for cleanup)
    const clearCache = () => {
        elementCache.clear();
    };
    
    // Public API
    return {
        updateElement,
        updateElements,
        updateLiveWeather,
        updateSnowData,
        updateWindrose,
        updateBatteryInfo,
        updateLiveIndicator,
        updateWeekForecast,
        updateDayForecast,
        switchChartRange,
        switchForecastType,
        clearCache,
        getCachedElement
    };
})();

// Make available globally if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherDOM;
}