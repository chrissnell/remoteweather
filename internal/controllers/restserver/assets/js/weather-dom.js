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
    
    // Air quality thresholds configuration
    // Map air quality levels to CSS classes
    const getAirQualityClass = (level) => {
        const classMap = {
            [WeatherUtils.AirQualityLevels.EXCELLENT]: 'air-quality-good',
            [WeatherUtils.AirQualityLevels.GOOD]: 'air-quality-good',
            [WeatherUtils.AirQualityLevels.FAIR]: 'air-quality-moderate',
            [WeatherUtils.AirQualityLevels.MODERATE]: 'air-quality-moderate',
            [WeatherUtils.AirQualityLevels.POOR]: 'air-quality-unhealthy-sensitive',
            [WeatherUtils.AirQualityLevels.UNHEALTHY]: 'air-quality-unhealthy',
            [WeatherUtils.AirQualityLevels.VERY_UNHEALTHY]: 'air-quality-very-unhealthy',
            [WeatherUtils.AirQualityLevels.HAZARDOUS]: 'air-quality-hazardous',
            [WeatherUtils.AirQualityLevels.DANGEROUS]: 'air-quality-hazardous',
            [WeatherUtils.AirQualityLevels.UNKNOWN]: ''
        };
        return classMap[level] || '';
    };

    // Format value helper
    const formatAirQualityValue = (value, decimals) => {
        if (value === null || value === undefined) return '--';
        return decimals === 0 ? Math.round(value).toString() : value.toFixed(decimals);
    };
    
    // Update air quality data display
    const updateAirQualityData = (airQualityData) => {
        if (!airQualityData) return;
        
        // Update values
        updateElements({
            'pm25': formatAirQualityValue(airQualityData.pm25, 1),
            'pm10': formatAirQualityValue(airQualityData.pm10, 1),
            'pm1': formatAirQualityValue(airQualityData.pm1, 1),
            'co2': formatAirQualityValue(airQualityData.co2, 0),
            'tvoc': formatAirQualityValue(airQualityData.tvoc, 0),
            'nox': formatAirQualityValue(airQualityData.nox, 0)
        });
        
        // Update status for each metric
        ['pm25', 'pm10', 'pm1', 'co2', 'tvoc', 'nox'].forEach(metric => {
            updateAirQualityStatus(metric, airQualityData[metric]);
        });
        
        // Update last updated time
        updateElement('air-quality-last-updated', `Updated: ${new Date().toLocaleTimeString()}`);
    };
    
    // Helper function to update air quality status with color using centralized system
    const updateAirQualityStatus = (metric, value) => {
        const statusElement = getCachedElement(`${metric}-status`);
        
        if (!statusElement || value === null || value === undefined) return;
        
        // Handle special metric name mapping
        let metricType = metric;
        if (metric === 'tvoc') metricType = 'tvocindex';
        if (metric === 'nox') metricType = 'noxindex';
        
        const level = WeatherUtils.getAirQualityLevel(metricType, value);
        const statusText = WeatherUtils.getAirQualityStatusText(level);
        const cssClass = getAirQualityClass(level);
        
        statusElement.textContent = statusText;
        statusElement.className = `metric-status ${cssClass}`;
        statusElement.style.color = WeatherUtils.getAirQualityColor(level);
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
    
    // Initialize tooltips for air quality metrics
    const initializeTooltips = () => {
        const tooltipData = {
            pm25: {
                title: 'PM2.5 - Fine Particulate Matter',
                content: `
                    <div class="tooltip-description">
                        <p><strong>What it measures:</strong> Tiny particles less than 2.5 micrometers in diameter - about 30 times smaller than the width of a human hair.</p>
                        <p><strong>Why it matters:</strong> These particles are small enough to penetrate deep into your lungs and even enter your bloodstream, potentially causing respiratory problems, heart disease, and other health issues. Common sources include vehicle exhaust, wildfires, and industrial emissions.</p>
                    </div>
                    <div class="tooltip-levels">
                        <div class="level-item">
                            <span class="level-dot" style="background: #2aa22a"></span>
                            <span class="level-range">0-12 µg/m³</span>
                            <span class="level-desc">Good - Air quality is satisfactory</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ffbf00"></span>
                            <span class="level-range">12-35 µg/m³</span>
                            <span class="level-desc">Moderate - Acceptable for most people</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff7e00"></span>
                            <span class="level-range">35-55 µg/m³</span>
                            <span class="level-desc">Unhealthy for Sensitive Groups</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff0000"></span>
                            <span class="level-range">55-150 µg/m³</span>
                            <span class="level-desc">Unhealthy - Everyone may experience effects</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #8f3f97"></span>
                            <span class="level-range">150-250 µg/m³</span>
                            <span class="level-desc">Very Unhealthy - Health warnings</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #7e0023"></span>
                            <span class="level-range">&gt;250 µg/m³</span>
                            <span class="level-desc">Hazardous - Emergency conditions</span>
                        </div>
                    </div>
                `
            },
            pm10: {
                title: 'PM10 - Coarse Particulate Matter',
                content: `
                    <div class="tooltip-description">
                        <p><strong>What it measures:</strong> Particles between 2.5 and 10 micrometers in diameter, including dust, pollen, and mold spores.</p>
                        <p><strong>Why it matters:</strong> While larger than PM2.5, these particles can still be inhaled and cause breathing difficulties, especially for people with asthma or allergies. Sources include construction sites, unpaved roads, and agricultural operations.</p>
                    </div>
                    <div class="tooltip-levels">
                        <div class="level-item">
                            <span class="level-dot" style="background: #2aa22a"></span>
                            <span class="level-range">0-54 µg/m³</span>
                            <span class="level-desc">Good - Air quality is satisfactory</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ffbf00"></span>
                            <span class="level-range">54-154 µg/m³</span>
                            <span class="level-desc">Moderate - Acceptable for most people</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff7e00"></span>
                            <span class="level-range">154-254 µg/m³</span>
                            <span class="level-desc">Unhealthy for Sensitive Groups</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff0000"></span>
                            <span class="level-range">254-354 µg/m³</span>
                            <span class="level-desc">Unhealthy - Everyone may experience effects</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #8f3f97"></span>
                            <span class="level-range">354-424 µg/m³</span>
                            <span class="level-desc">Very Unhealthy - Health warnings</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #7e0023"></span>
                            <span class="level-range">&gt;424 µg/m³</span>
                            <span class="level-desc">Hazardous - Emergency conditions</span>
                        </div>
                    </div>
                `
            },
            pm1: {
                title: 'PM1.0 - Ultra-fine Particulate Matter',
                content: `
                    <div class="tooltip-description">
                        <p><strong>What it measures:</strong> The smallest measurable particles, less than 1 micrometer in diameter - primarily from combustion sources.</p>
                        <p><strong>Why it matters:</strong> These ultra-fine particles can penetrate the deepest parts of your lungs and cross into other organs. They're especially concerning because they can carry toxic compounds and are linked to cardiovascular and neurological effects.</p>
                    </div>
                    <div class="tooltip-levels">
                        <div class="level-item">
                            <span class="level-dot" style="background: #2aa22a"></span>
                            <span class="level-range">0-10 µg/m³</span>
                            <span class="level-desc">Good - Minimal health risk</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ffbf00"></span>
                            <span class="level-range">10-25 µg/m³</span>
                            <span class="level-desc">Moderate - Low health risk</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff7e00"></span>
                            <span class="level-range">25-50 µg/m³</span>
                            <span class="level-desc">Elevated - Some health risk</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff0000"></span>
                            <span class="level-range">&gt;50 µg/m³</span>
                            <span class="level-desc">High - Significant health risk</span>
                        </div>
                    </div>
                    <div class="tooltip-note">Note: PM1.0 standards are still being developed by health organizations</div>
                `
            },
            co2: {
                title: 'CO₂ - Carbon Dioxide',
                content: `
                    <div class="tooltip-description">
                        <p><strong>What it measures:</strong> The concentration of carbon dioxide in the air, measured in parts per million (ppm).</p>
                        <p><strong>Why it matters:</strong> Indoor CO₂ levels indicate air freshness and ventilation quality. High levels can cause drowsiness, headaches, and reduced cognitive performance. Outdoor levels average around 420 ppm, while poorly ventilated indoor spaces can exceed 2000 ppm.</p>
                    </div>
                    <div class="tooltip-levels">
                        <div class="level-item">
                            <span class="level-dot" style="background: #2aa22a"></span>
                            <span class="level-range">400-800 ppm</span>
                            <span class="level-desc">Excellent - Fresh air, ideal conditions</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #90ee90"></span>
                            <span class="level-range">800-1000 ppm</span>
                            <span class="level-desc">Good - Acceptable indoor air quality</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ffbf00"></span>
                            <span class="level-range">1000-1500 ppm</span>
                            <span class="level-desc">Fair - Some stuffiness, ventilation recommended</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff7e00"></span>
                            <span class="level-range">1500-2000 ppm</span>
                            <span class="level-desc">Poor - Drowsiness possible, improve ventilation</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff0000"></span>
                            <span class="level-range">2000-5000 ppm</span>
                            <span class="level-desc">Very Poor - Headaches, increased heart rate</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #7e0023"></span>
                            <span class="level-range">&gt;5000 ppm</span>
                            <span class="level-desc">Dangerous - Immediate ventilation required</span>
                        </div>
                    </div>
                `
            },
            tvoc: {
                title: 'TVOC - Total Volatile Organic Compounds',
                content: `
                    <div class="tooltip-description">
                        <p><strong>What it measures:</strong> The combined level of various organic chemicals that easily evaporate at room temperature.</p>
                        <p><strong>Why it matters:</strong> VOCs come from paints, cleaning products, furniture, and building materials. They can cause eye/throat irritation, headaches, and long-term health effects. Good ventilation and choosing low-VOC products helps maintain healthy levels.</p>
                    </div>
                    <div class="tooltip-levels">
                        <div class="level-item">
                            <span class="level-dot" style="background: #2aa22a"></span>
                            <span class="level-range">0-65 Index</span>
                            <span class="level-desc">Excellent - Pure air</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #90ee90"></span>
                            <span class="level-range">65-220 Index</span>
                            <span class="level-desc">Good - No irritation or discomfort</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ffbf00"></span>
                            <span class="level-range">220-660 Index</span>
                            <span class="level-desc">Fair - Possible irritation with prolonged exposure</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff7e00"></span>
                            <span class="level-range">660-1430 Index</span>
                            <span class="level-desc">Poor - Irritation and discomfort possible</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff0000"></span>
                            <span class="level-range">1430-2200 Index</span>
                            <span class="level-desc">Bad - Strong irritation likely</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #7e0023"></span>
                            <span class="level-range">&gt;2200 Index</span>
                            <span class="level-desc">Very Bad - Toxic effects possible</span>
                        </div>
                    </div>
                    <div class="tooltip-note">TVOC Index is a relative measurement, not an absolute concentration</div>
                `
            },
            nox: {
                title: 'NOx - Nitrogen Oxides',
                content: `
                    <div class="tooltip-description">
                        <p><strong>What it measures:</strong> A group of gases (primarily NO and NO₂) commonly produced by vehicle emissions and combustion processes.</p>
                        <p><strong>Why it matters:</strong> NOx irritates airways and can worsen respiratory diseases like asthma. It's a key indicator of traffic-related air pollution and contributes to smog formation. Indoor sources include gas stoves and heaters.</p>
                    </div>
                    <div class="tooltip-levels">
                        <div class="level-item">
                            <span class="level-dot" style="background: #2aa22a"></span>
                            <span class="level-range">1-20 Index</span>
                            <span class="level-desc">Excellent - Pure air</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #90ee90"></span>
                            <span class="level-range">20-50 Index</span>
                            <span class="level-desc">Good - No health effects</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ffbf00"></span>
                            <span class="level-range">50-150 Index</span>
                            <span class="level-desc">Fair - Sensitive individuals may be affected</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff7e00"></span>
                            <span class="level-range">150-250 Index</span>
                            <span class="level-desc">Poor - Respiratory irritation possible</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #ff0000"></span>
                            <span class="level-range">250-400 Index</span>
                            <span class="level-desc">Bad - Significant health effects</span>
                        </div>
                        <div class="level-item">
                            <span class="level-dot" style="background: #7e0023"></span>
                            <span class="level-range">&gt;400 Index</span>
                            <span class="level-desc">Very Bad - Dangerous levels</span>
                        </div>
                    </div>
                    <div class="tooltip-note">NOx Index is a relative measurement primarily indicating traffic-related air pollution</div>
                `
            }
        };
        
        // Create tooltip container if it doesn't exist
        let tooltipContainer = document.getElementById('air-quality-tooltip');
        if (!tooltipContainer) {
            tooltipContainer = document.createElement('div');
            tooltipContainer.id = 'air-quality-tooltip';
            tooltipContainer.className = 'tooltip-container';
            tooltipContainer.style.display = 'none';
            document.body.appendChild(tooltipContainer);
        }
        
        // Add hover handlers to all info icons
        document.querySelectorAll('.info-icon').forEach(icon => {
            // Show tooltip on mouse enter
            // Function to show tooltip
            const showTooltip = (icon) => {
                const metric = icon.getAttribute('data-tooltip');
                const data = tooltipData[metric];
                
                if (data) {
                    // Update tooltip content - add close button for mobile
                    const isMobile = window.innerWidth <= 768;
                    var tooltipHTML = '<div class="tooltip-header">' +
                        '<h4>' + data.title + '</h4>' +
                        (isMobile ? '<span class="tooltip-close">×</span>' : '') +
                        '</div>' +
                        '<div class="tooltip-content">' +
                        data.content.trim() +
                        '</div>';
                    tooltipContainer.innerHTML = tooltipHTML;
                    
                    // Show tooltip first to measure its dimensions
                    tooltipContainer.style.display = 'block';
                    tooltipContainer.style.visibility = 'hidden'; // Hide while positioning
                    
                    // Position tooltip
                    const rect = icon.getBoundingClientRect();
                    const tooltipRect = tooltipContainer.getBoundingClientRect();
                    const tooltipHeight = tooltipRect.height;
                    const tooltipWidth = tooltipRect.width;
                    const viewportHeight = window.innerHeight;
                    const viewportWidth = window.innerWidth;
                    
                    if (isMobile) {
                        // Mobile: center horizontally
                        tooltipContainer.style.left = '10px';
                        tooltipContainer.style.right = '10px';
                        tooltipContainer.style.width = 'auto';
                        
                        // Calculate available space above and below
                        const spaceBelow = viewportHeight - rect.bottom - 10;
                        const spaceAbove = rect.top - 10;
                        
                        let topPos;
                        
                        // If there's enough space below, position below
                        if (spaceBelow >= tooltipHeight || spaceBelow > spaceAbove) {
                            topPos = rect.bottom + 10;
                            // If tooltip extends beyond viewport, constrain it
                            if (topPos + tooltipHeight > viewportHeight) {
                                topPos = viewportHeight - tooltipHeight - 10;
                            }
                        } else {
                            // Position above the icon
                            topPos = rect.top - tooltipHeight - 10;
                        }
                        
                        // Ensure tooltip stays within viewport bounds
                        topPos = Math.max(10, Math.min(topPos, viewportHeight - tooltipHeight - 10));
                        
                        tooltipContainer.style.top = topPos + 'px';
                        
                        // Scroll the icon into view if needed
                        if (topPos < window.scrollY || topPos + tooltipHeight > window.scrollY + viewportHeight) {
                            icon.scrollIntoView({ behavior: 'smooth', block: 'center' });
                        }
                    } else {
                        // Desktop: position with smart placement
                        let leftPos = rect.left;
                        let topPos = rect.bottom + 10;
                        
                        // Adjust horizontal position if tooltip would go off screen
                        if (leftPos + tooltipWidth > viewportWidth) {
                            leftPos = viewportWidth - tooltipWidth - 10;
                        }
                        
                        // Check if there's enough space below
                        const spaceBelow = viewportHeight - rect.bottom - 10;
                        const spaceAbove = rect.top - 10;
                        
                        if (spaceBelow < tooltipHeight && spaceAbove > spaceBelow) {
                            // Position above if more space there
                            topPos = rect.top - tooltipHeight - 10;
                        }
                        
                        // Ensure tooltip stays within viewport bounds
                        topPos = Math.max(10, Math.min(topPos, viewportHeight - tooltipHeight - 10));
                        leftPos = Math.max(10, leftPos);
                        
                        tooltipContainer.style.left = leftPos + 'px';
                        tooltipContainer.style.top = topPos + 'px';
                        tooltipContainer.style.width = '';
                        tooltipContainer.style.right = '';
                    }
                    
                    // Make visible after positioning
                    tooltipContainer.style.visibility = 'visible';
                    
                    // Add close button handler for mobile
                    if (isMobile) {
                        const closeBtn = tooltipContainer.querySelector('.tooltip-close');
                        if (closeBtn) {
                            closeBtn.addEventListener('click', () => {
                                tooltipContainer.style.display = 'none';
                            });
                        }
                    }
                }
            };
            
            // Mobile: click/touch events
            icon.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();
                if (window.innerWidth <= 768) {
                    // Close any existing tooltips first
                    if (tooltipContainer.style.display === 'block') {
                        tooltipContainer.style.display = 'none';
                        // Small delay to allow for closing animation
                        setTimeout(() => showTooltip(icon), 100);
                    } else {
                        showTooltip(icon);
                    }
                }
            });
            
            // Desktop: hover events
            icon.addEventListener('mouseenter', () => {
                if (window.innerWidth > 768) {
                    showTooltip(icon);
                }
            });
            
            // Hide tooltip on mouse leave (desktop only)
            icon.addEventListener('mouseleave', () => {
                if (window.innerWidth > 768) {
                    // Add a small delay to prevent flickering when moving between icon and tooltip
                    setTimeout(() => {
                        // Check if mouse is not over the tooltip itself
                        if (!tooltipContainer.matches(':hover')) {
                            tooltipContainer.style.display = 'none';
                        }
                    }, 100);
                }
            });
        });
        
        // Keep tooltip visible when hovering over it
        if (tooltipContainer) {
            tooltipContainer.addEventListener('mouseenter', () => {
                tooltipContainer.style.display = 'block';
            });
            
            tooltipContainer.addEventListener('mouseleave', () => {
                tooltipContainer.style.display = 'none';
            });
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
        updateAirQualityData,
        updateWindrose,
        updateBatteryInfo,
        updateLiveIndicator,
        updateWeekForecast,
        updateDayForecast,
        switchChartRange,
        switchForecastType,
        clearCache,
        getCachedElement,
        initializeTooltips
    };
})();

// Make available globally if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherDOM;
}