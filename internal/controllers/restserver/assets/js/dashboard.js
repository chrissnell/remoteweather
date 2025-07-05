// Weather Management Portal JavaScript
class WeatherDashboard {
    constructor() {
        this.map = null;
        this.stations = [];
        this.markers = [];
        this.refreshInterval = null;
        this.isLoading = false;
        
        // Initialize dashboard when DOM is ready
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
    
    async loadStationData() {
        if (this.isLoading) return;
        
        this.isLoading = true;
        this.showLoading(true);
        
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
            
        } catch (error) {
            console.error('Error loading station data:', error);
            this.showError('Unable to load weather station data');
        } finally {
            this.isLoading = false;
            this.showLoading(false);
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
        // Clear existing markers
        this.markers.forEach(marker => this.map.removeLayer(marker));
        this.markers = [];
        
        // Add markers for each station
        this.stations.forEach(station => {
            if (station.latitude && station.longitude) {
                const marker = this.createStationMarker(station);
                this.markers.push(marker);
                marker.addTo(this.map);
            }
        });
        
        // Fit map to show all markers if we have stations
        if (this.markers.length > 0) {
            const group = new L.featureGroup(this.markers);
            this.map.fitBounds(group.getBounds().pad(0.1));
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
        
        // Create popup content
        const popupContent = this.createPopupContent(station);
        marker.bindPopup(popupContent, {
            maxWidth: 320,
            className: 'weather-popup'
        });
        
        return marker;
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
            // Weather data grid
            const grid = document.createElement('div');
            grid.className = 'popup-weather-grid';
            
            const weatherItems = [
                { label: 'Temperature', value: this.formatTemperature(station.weather.otemp) },
                { label: 'Humidity', value: this.formatHumidity(station.weather.ohum) },
                { label: 'Pressure', value: this.formatPressure(station.weather.barometerPoint) },
                { label: 'Wind Speed', value: this.formatWindSpeed(station.weather.windSpeedPoint) }
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
            
            // Wind rose
            const windroseContainer = this.createWindrose(station.weather);
            container.appendChild(windroseContainer);
        } else {
            // No data available
            const noData = document.createElement('div');
            noData.style.textAlign = 'center';
            noData.style.color = '#7f8c8d';
            noData.style.fontStyle = 'italic';
            noData.textContent = 'Weather data unavailable';
            container.appendChild(noData);
        }
        
        return container;
    }
    
    createWindrose(weatherData) {
        const container = document.createElement('div');
        container.className = 'popup-windrose-container';
        
        const windrose = document.createElement('div');
        windrose.className = 'popup-windrose';
        
        // Wind direction arrow
        const arrow = document.createElement('div');
        arrow.className = 'popup-windrose-arrow';
        
        // Get wind direction and rotate arrow
        const windDirection = this.getWindDirection(weatherData);
        if (windDirection !== null) {
            arrow.style.transform = `translateY(-8px) rotate(${windDirection}deg)`;
        }
        
        // Center dot
        const center = document.createElement('div');
        center.className = 'popup-windrose-center';
        
        // Compass labels
        const labels = document.createElement('div');
        labels.className = 'popup-windrose-labels';
        
        const directions = ['N', 'S', 'E', 'W'];
        const classes = ['popup-windrose-n', 'popup-windrose-s', 'popup-windrose-e', 'popup-windrose-w'];
        
        directions.forEach((dir, index) => {
            const label = document.createElement('div');
            label.className = classes[index];
            label.textContent = dir;
            labels.appendChild(label);
        });
        
        windrose.appendChild(arrow);
        windrose.appendChild(center);
        windrose.appendChild(labels);
        
        // Wind info
        const windInfo = document.createElement('div');
        windInfo.className = 'popup-wind-info';
        
        const windSpeed = document.createElement('div');
        windSpeed.className = 'popup-wind-speed';
        windSpeed.textContent = this.formatWindSpeed(weatherData.windSpeedPoint);
        
        const windDir = document.createElement('div');
        windDir.className = 'popup-wind-direction';
        windDir.textContent = this.getWindDirectionText(weatherData);
        
        windInfo.appendChild(windSpeed);
        windInfo.appendChild(windDir);
        
        container.appendChild(windrose);
        container.appendChild(windInfo);
        
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
        // Refresh button
        const refreshBtn = document.getElementById('refresh-btn');
        refreshBtn.addEventListener('click', () => {
            this.loadStationData();
        });
        
        // Disable refresh button during loading
        this.map.on('movestart', () => {
            if (this.isLoading) {
                refreshBtn.disabled = true;
            }
        });
        
        this.map.on('moveend', () => {
            refreshBtn.disabled = false;
        });
    }
    
    startAutoRefresh() {
        // Refresh data every 5 minutes
        this.refreshInterval = setInterval(() => {
            this.loadStationData();
        }, 5 * 60 * 1000);
    }
    
    showLoading(show) {
        const loading = document.getElementById('loading-indicator');
        const refreshBtn = document.getElementById('refresh-btn');
        
        if (show) {
            loading.style.display = 'block';
            refreshBtn.disabled = true;
            refreshBtn.textContent = 'Loading...';
        } else {
            loading.style.display = 'none';
            refreshBtn.disabled = false;
            refreshBtn.textContent = 'Refresh Data';
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
        if (weatherData.windDirectionVectorPoint) {
            return parseFloat(weatherData.windDirectionVectorPoint);
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

// Initialize dashboard when script loads
const dashboard = new WeatherDashboard(); 