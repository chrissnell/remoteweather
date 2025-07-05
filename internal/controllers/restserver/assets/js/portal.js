// Weather Management Portal JavaScript
class WeatherPortal {
    constructor() {
        this.map = null;
        this.stations = [];
        this.markers = [];
        this.refreshInterval = null;
        this.statusInterval = null;
        this.isLoading = false;
        this.hasInitializedBounds = false;
        this.lastRefreshTime = null;
        this.secondsSinceRefresh = 0;
        
        // Initialize portal when DOM is ready
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', () => this.init());
        } else {
            this.init();
        }
    }
    
    init() {
        this.initializeMap();
        this.loadStationData();
        this.setupEventListeners();
        this.startAutoRefresh();
        this.startStatusTimer();
    }
    
    initializeMap() {
        // Initialize Leaflet map
        this.map = L.map('weather-map').setView([39.8283, -98.5795], 4); // Center of USA
        
        // Add OpenStreetMap tiles
        L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            attribution: '© OpenStreetMap contributors',
            maxZoom: 18
        }).addTo(this.map);
        
        // Try to get user's location for better initial view
        if (navigator.geolocation) {
            navigator.geolocation.getCurrentPosition(
                (position) => {
                    const lat = position.coords.latitude;
                    const lon = position.coords.longitude;
                    this.map.setView([lat, lon], 8);
                },
                (error) => {
                    console.log('Geolocation not available, using default view');
                }
            );
        }
    }
    
    async loadStationData(showLoadingIndicator = true) {
        if (this.isLoading) return;
        
        this.isLoading = true;
        if (showLoadingIndicator) {
            this.showLoading(true);
        }
        
        try {
            // Get all unique stations from the API
            const response = await fetch('/api/stations');
            if (!response.ok) {
                throw new Error('Failed to fetch station data');
            }
            
            const stationData = await response.json();
            this.stations = stationData;
            
            // Load weather data for all stations
            await this.loadWeatherData();
            
            // Update map markers and sidebar
            this.updateMapMarkers();
            this.updateStationList();
            
            // Reset the refresh timer on successful update
            this.resetRefreshTimer();
            
        } catch (error) {
            console.error('Error loading station data:', error);
            if (showLoadingIndicator) {
                this.showError('Unable to load weather station data');
            }
        } finally {
            this.isLoading = false;
            if (showLoadingIndicator) {
                this.showLoading(false);
            }
        }
    }
    
    async loadWeatherData() {
        const weatherPromises = this.stations.map(async (station) => {
            try {
                const response = await fetch(`/latest?station=${station.name}`);
                if (response.ok) {
                    const weatherData = await response.json();
                    station.weather = weatherData;
                    station.status = 'online';
                    station.lastUpdate = new Date();
                } else {
                    station.status = 'offline';
                    station.weather = null;
                }
            } catch (error) {
                console.error(`Error loading weather for ${station.name}:`, error);
                station.status = 'error';
                station.weather = null;
            }
        });
        
        await Promise.all(weatherPromises);
    }
    
    updateMapMarkers() {
        // Track existing markers by station name
        const existingMarkers = new Map();
        this.markers.forEach(marker => {
            if (marker.stationName) {
                existingMarkers.set(marker.stationName, marker);
            }
        });
        
        const newMarkers = [];
        
        // Process each station
        this.stations.forEach(station => {
            if (station.latitude && station.longitude) {
                const existingMarker = existingMarkers.get(station.name);
                
                if (existingMarker) {
                    // Update existing marker
                    this.updateMarkerContent(existingMarker, station);
                    newMarkers.push(existingMarker);
                    existingMarkers.delete(station.name);
                } else {
                    // Create new marker
                    const marker = this.createStationMarker(station);
                    newMarkers.push(marker);
                    marker.addTo(this.map);
                }
            }
        });
        
        // Remove markers for stations that no longer exist
        existingMarkers.forEach(marker => {
            this.map.removeLayer(marker);
        });
        
        this.markers = newMarkers;
        
        // Fit map to show all markers if we have stations (only on first load)
        if (this.markers.length > 0 && !this.hasInitializedBounds) {
            const group = new L.featureGroup(this.markers);
            this.map.fitBounds(group.getBounds().pad(0.1));
            this.hasInitializedBounds = true;
        }
    }
    
    createStationMarker(station) {
        // Create custom marker icon based on status
        const iconClass = this.getMarkerClass(station);
        const iconHtml = `<div class="weather-station-marker ${iconClass}"></div>`;
        
        const customIcon = L.divIcon({
            html: iconHtml,
            className: 'custom-marker',
            iconSize: [24, 24],
            iconAnchor: [12, 12],
            popupAnchor: [0, -12]
        });
        
        const marker = L.marker([station.latitude, station.longitude], {
            icon: customIcon
        });
        
        // Store station name for tracking
        marker.stationName = station.name;
        
        // Create popup content
        const popupContent = this.createPopupContent(station);
        marker.bindPopup(popupContent, {
            maxWidth: 320,
            className: 'weather-popup'
        });
        
        return marker;
    }
    
    updateMarkerContent(marker, station) {
        // Update marker icon based on current status
        const iconClass = this.getMarkerClass(station);
        const iconHtml = `<div class="weather-station-marker ${iconClass}"></div>`;
        
        const customIcon = L.divIcon({
            html: iconHtml,
            className: 'custom-marker',
            iconSize: [24, 24],
            iconAnchor: [12, 12],
            popupAnchor: [0, -12]
        });
        
        marker.setIcon(customIcon);
        
        // Update popup content
        const popupContent = this.createPopupContent(station);
        marker.setPopupContent(popupContent);
    }
    
    getMarkerClass(station) {
        if (station.status === 'offline') return 'offline';
        if (station.status === 'error') return 'error';
        
        // Check for data freshness (warning if older than 1 hour)
        if (station.weather && station.lastUpdate) {
            const ageMinutes = (new Date() - station.lastUpdate) / (1000 * 60);
            if (ageMinutes > 60) return 'warning';
        }
        
        return 'online';
    }
    
    createPopupContent(station) {
        const container = document.createElement('div');
        container.className = 'weather-popup';
        
        // Header
        const header = document.createElement('div');
        header.className = 'popup-header';
        
        const stationName = document.createElement('div');
        stationName.className = 'popup-station-name';
        stationName.textContent = station.name;
        
        const timestamp = document.createElement('div');
        timestamp.className = 'popup-timestamp';
        timestamp.textContent = station.lastUpdate ? 
            moment(station.lastUpdate).format('MMM D, h:mm A') : 'No data';
        
        header.appendChild(stationName);
        header.appendChild(timestamp);
        container.appendChild(header);
        
        if (station.weather) {
            // Wind rose at the top
            const windroseContainer = this.createWindrose(station.weather);
            container.appendChild(windroseContainer);
            
            // Weather data grid
            const grid = document.createElement('div');
            grid.className = 'popup-weather-grid';
            
            const weatherItems = [
                { label: 'Temperature', value: this.formatTemperature(station.weather.otemp) },
                { label: 'Humidity', value: this.formatHumidity(station.weather.ohum) },
                { label: 'Pressure', value: this.formatPressure(station.weather.bar) },
                { label: 'Wind Speed', value: this.formatWindSpeed(station.weather.winds) }
            ];
            
            weatherItems.forEach(item => {
                const itemDiv = document.createElement('div');
                itemDiv.className = 'popup-weather-item';
                
                const label = document.createElement('div');
                label.className = 'popup-weather-label';
                label.textContent = item.label;
                
                const value = document.createElement('div');
                value.className = 'popup-weather-value';
                value.textContent = item.value;
                
                itemDiv.appendChild(label);
                itemDiv.appendChild(value);
                grid.appendChild(itemDiv);
            });
            
            container.appendChild(grid);
        } else {
            // No data available
            const noData = document.createElement('div');
            noData.style.textAlign = 'center';
            noData.style.color = '#7f8c8d';
            noData.style.fontStyle = 'italic';
            noData.textContent = 'Weather data unavailable';
            container.appendChild(noData);
        }
        
        // Add website link if available
        if (station.website && station.website.hostname) {
            const websiteLink = document.createElement('div');
            websiteLink.className = 'popup-website-link';
            
            const link = document.createElement('a');
            
            // Construct URL with proper protocol and port
            let url = `${station.website.protocol}://${station.website.hostname}`;
            
            // Only include port if it's not the standard port for the protocol
            const isStandardPort = (station.website.protocol === 'http' && station.website.port === 80) ||
                                   (station.website.protocol === 'https' && station.website.port === 443);
            
            if (!isStandardPort) {
                url += `:${station.website.port}`;
            }
            
            link.href = url;
            link.target = '_blank';
            link.rel = 'noopener noreferrer';
            
            const websiteName = station.website.page_title || station.website.name;
            link.textContent = `↗ Open ${websiteName}`;
            
            websiteLink.appendChild(link);
            container.appendChild(websiteLink);
        }
        
        return container;
    }
    
    createWindrose(weatherData) {
        const container = document.createElement('div');
        container.className = 'popup-windrosecontainer';
        
        const windroseBox = document.createElement('div');
        windroseBox.className = 'popup-windrose-box';
        
        const divBlock3 = document.createElement('div');
        divBlock3.className = 'popup-div-block-3';
        
        // Wind cardinal direction
        const windCardinalDir = document.createElement('div');
        windCardinalDir.className = 'popup-wind-cardinal-dir';
        windCardinalDir.textContent = this.getWindDirectionText(weatherData);
        
        // Wind speed
        const windSpeed = document.createElement('div');
        windSpeed.className = 'popup-wind-speed';
        windSpeed.textContent = weatherData.winds ? Math.round(parseFloat(weatherData.winds)) : '--';
        
        // Windrose box container
        const windroseBoxContainer = document.createElement('div');
        windroseBoxContainer.className = 'popup-windrose-box-container';
        
        // Windrose circle (this rotates)
        const windroseCircle = document.createElement('div');
        windroseCircle.className = 'popup-windrose-circle';
        
        // Wind direction arrow (positioned on edge)
        const arrow = document.createElement('div');
        arrow.className = 'popup-windrose-arrow';
        
        // Get wind direction and rotate the entire circle
        const windDirection = this.getWindDirection(weatherData);
        if (windDirection !== null) {
            windroseCircle.style.transform = `rotate(${windDirection}deg)`;
        }
        
        windroseCircle.appendChild(arrow);
        windroseBoxContainer.appendChild(windroseCircle);
        
        // Add wind speed and cardinal direction inside the circle
        windroseBoxContainer.appendChild(windCardinalDir);
        windroseBoxContainer.appendChild(windSpeed);
        
        divBlock3.appendChild(windroseBoxContainer);
        
        windroseBox.appendChild(divBlock3);
        container.appendChild(windroseBox);
        
        return container;
    }
    
    updateStationList() {
        const listContent = document.getElementById('station-list-content');
        listContent.innerHTML = '';
        
        this.stations.forEach(station => {
            const item = document.createElement('div');
            item.className = 'station-item';
            
            const name = document.createElement('div');
            name.className = 'station-name';
            name.textContent = station.name;
            
            const status = document.createElement('div');
            status.className = 'station-status';
            status.textContent = this.getStatusText(station);
            
            const temp = document.createElement('div');
            temp.className = 'station-temp';
            temp.textContent = station.weather ? 
                this.formatTemperature(station.weather.otemp) : '--';
            
            item.appendChild(name);
            item.appendChild(status);
            item.appendChild(temp);
            
            // Click handler to focus on station
            item.addEventListener('click', () => {
                this.focusOnStation(station);
                // Update active state
                document.querySelectorAll('.station-item').forEach(i => i.classList.remove('active'));
                item.classList.add('active');
            });
            
            listContent.appendChild(item);
        });
    }
    
    focusOnStation(station) {
        if (station.latitude && station.longitude) {
            this.map.setView([station.latitude, station.longitude], 12);
            
            // Find and open the marker popup
            const marker = this.markers.find(m => 
                m.getLatLng().lat === station.latitude && 
                m.getLatLng().lng === station.longitude
            );
            
            if (marker) {
                marker.openPopup();
            }
        }
    }
    
    setupEventListeners() {
        // No manual refresh button needed - auto-refresh handles everything
        // Could add other event listeners here if needed in the future
    }
    
    startAutoRefresh() {
        // Refresh data every 5 seconds
        this.refreshInterval = setInterval(() => {
            this.loadStationData(false); // Don't show loading indicator for auto-refresh
        }, 5 * 1000);
    }
    
    startStatusTimer() {
        // Update status every second
        this.statusInterval = setInterval(() => {
            this.updateRefreshStatus();
        }, 1000);
    }
    
    updateRefreshStatus() {
        const statusElement = document.getElementById('refresh-status');
        if (!statusElement) return;
        
        if (this.lastRefreshTime) {
            this.secondsSinceRefresh = Math.floor((Date.now() - this.lastRefreshTime) / 1000);
            statusElement.textContent = `Updated ${this.secondsSinceRefresh}s ago`;
        } else {
            statusElement.textContent = 'Loading...';
        }
    }
    
    resetRefreshTimer() {
        this.lastRefreshTime = Date.now();
        this.secondsSinceRefresh = 0;
        this.updateRefreshStatus();
    }
    
    showLoading(show) {
        const loading = document.getElementById('loading-indicator');
        
        if (show) {
            loading.style.display = 'block';
        } else {
            loading.style.display = 'none';
        }
    }
    
    showError(message) {
        const errorDiv = document.getElementById('error-message');
        errorDiv.querySelector('p').textContent = message;
        errorDiv.style.display = 'block';
        
        // Auto-hide after 5 seconds
        setTimeout(() => {
            errorDiv.style.display = 'none';
        }, 5000);
    }
    
    // Utility methods for formatting data
    formatTemperature(temp) {
        return temp ? `${Math.round(parseFloat(temp))}°F` : '--';
    }
    
    formatHumidity(humidity) {
        return humidity ? `${Math.round(parseFloat(humidity))}%` : '--';
    }
    
    formatPressure(pressure) {
        return pressure ? `${parseFloat(pressure).toFixed(2)} inHg` : '--';
    }
    
    formatWindSpeed(speed) {
        return speed ? `${Math.round(parseFloat(speed))} mph` : '--';
    }
    
    getWindDirection(weatherData) {
        // Extract wind direction from weather data
        if (weatherData.windd) {
            return parseFloat(weatherData.windd);
        }
        return null;
    }
    
    getWindDirectionText(weatherData) {
        const dir = this.getWindDirection(weatherData);
        if (dir === null) return '--';
        
        const directions = ['N', 'NNE', 'NE', 'ENE', 'E', 'ESE', 'SE', 'SSE', 
                          'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW'];
        const index = Math.round(dir / 22.5) % 16;
        return directions[index];
    }
    
    getStatusText(station) {
        if (station.status === 'offline') return 'Offline';
        if (station.status === 'error') return 'Error';
        
        if (station.lastUpdate) {
            const ageMinutes = (new Date() - station.lastUpdate) / (1000 * 60);
            if (ageMinutes > 60) return 'Stale data';
            return 'Online';
        }
        
        return 'Unknown';
    }
}

// Initialize portal when script loads
const portal = new WeatherPortal(); 