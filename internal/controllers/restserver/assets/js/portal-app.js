// Weather Portal Main Application
// Orchestrates all modules and manages application state

class WeatherPortalApp {
    constructor() {
        // Application state
        this.stations = [];
        this.refreshInterval = null;
        this.statusInterval = null;
        this.isLoading = false;
        this.lastRefreshTime = null;
        this.secondsSinceRefresh = 0;
        this.currentDisplayType = 'temperature'; // Default display type
        
        // Intervals configuration
        this.INTERVALS = {
            DATA_REFRESH: 5000,  // 5 seconds
            STATUS_UPDATE: 1000  // 1 second
        };
    }
    
    // Initialize the application
    init() {
        // Initialize DOM
        PortalDOM.init();
        
        // Initialize map
        if (window.PortalMap) {
            window.PortalMap.initializeMap();
        }
        
        // Load initial data
        this.loadStationData();
        
        // Setup event listeners
        this.setupEventListeners();
        
        // Start auto-refresh
        this.startAutoRefresh();
        this.startStatusTimer();
    }
    
    // Load station data
    async loadStationData(showLoadingIndicator = true) {
        if (this.isLoading) return;
        
        this.isLoading = true;
        if (showLoadingIndicator) {
            PortalDOM.showLoading(true);
        }
        
        try {
            // Fetch all station data with weather
            this.stations = await PortalDataService.fetchAllStationData();
            
            // Update UI components
            if (window.PortalMap) {
                window.PortalMap.updateMapMarkers(this.stations, this.currentDisplayType);
            }
            PortalDOM.updateStationList(this.stations, (station) => this.focusOnStation(station));
            
            // Reset the refresh timer on successful update
            this.resetRefreshTimer();
            
        } catch (error) {
            console.error('Error loading station data:', error);
            if (showLoadingIndicator) {
                PortalDOM.showError('Unable to load weather station data');
            }
        } finally {
            this.isLoading = false;
            if (showLoadingIndicator) {
                PortalDOM.showLoading(false);
            }
        }
    }
    
    // Focus on a specific station
    focusOnStation(station) {
        if (window.PortalMap) {
            window.PortalMap.focusOnStation(station);
        }
        
        // If air quality display is active, show the air quality modal
        if (this.currentDisplayType === 'airquality') {
            this.showAirQualityModal(station);
        }
    }
    
    // Show air quality modal for a station
    showAirQualityModal(station) {
        const modal = document.getElementById('air-quality-modal');
        const stationNameElement = document.getElementById('air-quality-station-name');
        const dataContainer = document.getElementById('air-quality-data');
        const noDataElement = document.getElementById('air-quality-no-data');
        
        if (!modal) return;
        
        // Update station name
        stationNameElement.textContent = `${station.name} - Air Quality`;
        
        // Check if station has air quality data
        if (station.weather && this.hasAirQualityData(station.weather)) {
            // Hide no data message
            noDataElement.style.display = 'none';
            dataContainer.querySelector('.air-quality-grid').style.display = 'grid';
            
            // Update air quality values
            this.updateAirQualityDisplay(station.weather);
        } else {
            // Show no data message
            dataContainer.querySelector('.air-quality-grid').style.display = 'none';
            noDataElement.style.display = 'block';
        }
        
        // Show modal
        modal.style.display = 'block';
    }
    
    // Check if weather data has air quality measurements
    hasAirQualityData(weather) {
        return weather.pm25 !== null && weather.pm25 !== undefined && weather.pm25 > 0 ||
               weather.pm10_in_aqin !== null && weather.pm10_in_aqin !== undefined && weather.pm10_in_aqin > 0 ||
               weather.co2 !== null && weather.co2 !== undefined && weather.co2 > 0;
    }
    
    // Update air quality display with weather data
    updateAirQualityDisplay(weather) {
        // Update PM2.5
        const pm25Element = document.getElementById('aq-pm25');
        const pm25StatusElement = document.getElementById('aq-pm25-status');
        if (weather.pm25 !== null && weather.pm25 !== undefined) {
            pm25Element.textContent = weather.pm25.toFixed(1);
            pm25StatusElement.textContent = this.getAirQualityStatus('pm25', weather.pm25);
            pm25StatusElement.className = 'metric-status ' + this.getAirQualityClass('pm25', weather.pm25);
        }
        
        // Update PM10 (using pm10_in_aqin as approximation)
        const pm10Element = document.getElementById('aq-pm10');
        const pm10StatusElement = document.getElementById('aq-pm10-status');
        if (weather.pm10_in_aqin !== null && weather.pm10_in_aqin !== undefined) {
            pm10Element.textContent = weather.pm10_in_aqin.toFixed(1);
            pm10StatusElement.textContent = this.getAirQualityStatus('pm10', weather.pm10_in_aqin);
            pm10StatusElement.className = 'metric-status ' + this.getAirQualityClass('pm10', weather.pm10_in_aqin);
        }
        
        // Update CO2
        const co2Element = document.getElementById('aq-co2');
        const co2StatusElement = document.getElementById('aq-co2-status');
        if (weather.co2 !== null && weather.co2 !== undefined) {
            co2Element.textContent = Math.round(weather.co2);
            co2StatusElement.textContent = this.getAirQualityStatus('co2', weather.co2);
            co2StatusElement.className = 'metric-status ' + this.getAirQualityClass('co2', weather.co2);
        }
        
        // Update AQI PM2.5
        const aqiPm25Element = document.getElementById('aq-aqi-pm25');
        const aqiPm25StatusElement = document.getElementById('aq-aqi-pm25-status');
        if (weather.aqi_pm25_aqin !== null && weather.aqi_pm25_aqin !== undefined) {
            aqiPm25Element.textContent = weather.aqi_pm25_aqin;
            aqiPm25StatusElement.textContent = this.getAQIStatus(weather.aqi_pm25_aqin);
            aqiPm25StatusElement.className = 'metric-status ' + this.getAQIClass(weather.aqi_pm25_aqin);
        }
        
        // Update AQI PM10
        const aqiPm10Element = document.getElementById('aq-aqi-pm10');
        const aqiPm10StatusElement = document.getElementById('aq-aqi-pm10-status');
        if (weather.aqi_pm10_aqin !== null && weather.aqi_pm10_aqin !== undefined) {
            aqiPm10Element.textContent = weather.aqi_pm10_aqin;
            aqiPm10StatusElement.textContent = this.getAQIStatus(weather.aqi_pm10_aqin);
            aqiPm10StatusElement.className = 'metric-status ' + this.getAQIClass(weather.aqi_pm10_aqin);
        }
    }
    
    // Get air quality status text
    getAirQualityStatus(metric, value) {
        if (value === null || value === undefined) return '--';
        
        const thresholds = {
            pm25: [
                { limit: 12, status: 'Good' },
                { limit: 35, status: 'Moderate' },
                { limit: 55, status: 'Unhealthy (Sensitive)' },
                { limit: 150, status: 'Unhealthy' },
                { limit: 250, status: 'Very Unhealthy' },
                { limit: Infinity, status: 'Hazardous' }
            ],
            pm10: [
                { limit: 54, status: 'Good' },
                { limit: 154, status: 'Moderate' },
                { limit: 254, status: 'Unhealthy (Sensitive)' },
                { limit: 354, status: 'Unhealthy' },
                { limit: 424, status: 'Very Unhealthy' },
                { limit: Infinity, status: 'Hazardous' }
            ],
            co2: [
                { limit: 800, status: 'Excellent' },
                { limit: 1000, status: 'Good' },
                { limit: 1500, status: 'Fair' },
                { limit: 2000, status: 'Poor' },
                { limit: 5000, status: 'Very Poor' },
                { limit: Infinity, status: 'Dangerous' }
            ]
        };
        
        const levels = thresholds[metric];
        if (!levels) return '--';
        
        for (const level of levels) {
            if (value < level.limit) {
                return level.status;
            }
        }
        return '--';
    }
    
    // Get air quality CSS class
    getAirQualityClass(metric, value) {
        if (value === null || value === undefined) return '';
        
        const thresholds = {
            pm25: [
                { limit: 12, class: 'air-quality-good' },
                { limit: 35, class: 'air-quality-moderate' },
                { limit: 55, class: 'air-quality-unhealthy-sensitive' },
                { limit: 150, class: 'air-quality-unhealthy' },
                { limit: 250, class: 'air-quality-very-unhealthy' },
                { limit: Infinity, class: 'air-quality-hazardous' }
            ],
            pm10: [
                { limit: 54, class: 'air-quality-good' },
                { limit: 154, class: 'air-quality-moderate' },
                { limit: 254, class: 'air-quality-unhealthy-sensitive' },
                { limit: 354, class: 'air-quality-unhealthy' },
                { limit: 424, class: 'air-quality-very-unhealthy' },
                { limit: Infinity, class: 'air-quality-hazardous' }
            ],
            co2: [
                { limit: 1000, class: 'air-quality-good' },
                { limit: 1500, class: 'air-quality-moderate' },
                { limit: 2000, class: 'air-quality-unhealthy-sensitive' },
                { limit: 5000, class: 'air-quality-unhealthy' },
                { limit: Infinity, class: 'air-quality-hazardous' }
            ]
        };
        
        const levels = thresholds[metric];
        if (!levels) return '';
        
        for (const level of levels) {
            if (value < level.limit) {
                return level.class;
            }
        }
        return '';
    }
    
    // Get AQI status text
    getAQIStatus(value) {
        if (value === null || value === undefined) return '--';
        
        if (value <= 50) return 'Good';
        if (value <= 100) return 'Moderate';
        if (value <= 150) return 'Unhealthy (Sensitive)';
        if (value <= 200) return 'Unhealthy';
        if (value <= 300) return 'Very Unhealthy';
        return 'Hazardous';
    }
    
    // Get AQI CSS class
    getAQIClass(value) {
        if (value === null || value === undefined) return '';
        
        if (value <= 50) return 'air-quality-good';
        if (value <= 100) return 'air-quality-moderate';
        if (value <= 150) return 'air-quality-unhealthy-sensitive';
        if (value <= 200) return 'air-quality-unhealthy';
        if (value <= 300) return 'air-quality-very-unhealthy';
        return 'air-quality-hazardous';
    }
    
    // Setup event listeners
    setupEventListeners() {
        // Setup data display button handlers
        PortalDOM.setupDataDisplayButtons((dataType) => {
            this.currentDisplayType = dataType;
            // Update all markers with new display type
            if (window.PortalMap) {
                window.PortalMap.updateMapMarkers(this.stations, this.currentDisplayType);
            }
            
            // If air quality is selected, prompt user to click on a station
            if (dataType === 'airquality') {
                // Show instruction or do nothing - user needs to click on station
            }
        });
        
        // Setup modal close button
        const modal = document.getElementById('air-quality-modal');
        const closeBtn = modal ? modal.querySelector('.modal-close') : null;
        if (closeBtn) {
            closeBtn.addEventListener('click', () => {
                modal.style.display = 'none';
            });
        }
        
        // Close modal when clicking outside of it
        if (modal) {
            window.addEventListener('click', (event) => {
                if (event.target === modal) {
                    modal.style.display = 'none';
                }
            });
        }
    }
    
    // Start auto-refresh timer
    startAutoRefresh() {
        this.refreshInterval = setInterval(() => {
            this.loadStationData(false); // Don't show loading indicator for auto-refresh
        }, this.INTERVALS.DATA_REFRESH);
    }
    
    // Start status update timer
    startStatusTimer() {
        this.statusInterval = setInterval(() => {
            this.updateRefreshStatus();
        }, this.INTERVALS.STATUS_UPDATE);
    }
    
    // Update refresh status display
    updateRefreshStatus() {
        if (this.lastRefreshTime) {
            this.secondsSinceRefresh = Math.floor((Date.now() - this.lastRefreshTime) / 1000);
            PortalDOM.updateRefreshStatus(`Updated ${this.secondsSinceRefresh}s ago`);
        } else {
            PortalDOM.updateRefreshStatus('Loading...');
        }
    }
    
    // Reset refresh timer
    resetRefreshTimer() {
        this.lastRefreshTime = Date.now();
        this.secondsSinceRefresh = 0;
        this.updateRefreshStatus();
    }
    
    // Cleanup method (useful for SPA navigation)
    destroy() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        if (this.statusInterval) {
            clearInterval(this.statusInterval);
        }
    }
}

// Initialize the application when DOM is ready
let portalApp = null;

function initializePortal() {
    portalApp = new WeatherPortalApp();
    portalApp.init();
    // Make portalApp globally accessible for marker click handlers
    window.portalApp = portalApp;
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializePortal);
} else {
    initializePortal();
}

// Export for use in other contexts
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherPortalApp;
}