// Weather Portal Map Management
// Handles all Leaflet map operations, markers, and popups

const PortalMap = {
    map: null,
    markers: [],
    hasInitializedBounds: false,
    appInstance: null, // Store reference to app instance for callbacks

    // Initialize the Leaflet map
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
    },
    
    // Set the app instance for callbacks
    setAppInstance(app) {
        this.appInstance = app;
    },

    // Update all map markers with station data
    updateMapMarkers(stations, currentDisplayType) {
        // Track existing markers by station name
        const existingMarkers = new Map();
        this.markers.forEach(marker => {
            if (marker.stationName) {
                existingMarkers.set(marker.stationName, marker);
            }
        });
        
        const newMarkers = [];
        
        // Process each station
        stations.forEach(station => {
            if (station.latitude && station.longitude) {
                const existingMarker = existingMarkers.get(station.name);
                
                if (existingMarker) {
                    // Update existing marker
                    this.updateMarkerContent(existingMarker, station, currentDisplayType);
                    newMarkers.push(existingMarker);
                    existingMarkers.delete(station.name);
                } else {
                    // Create new marker
                    const marker = this.createStationMarker(station, currentDisplayType);
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
    },

    // Create a new marker for a station
    createStationMarker(station, currentDisplayType) {
        // Get data value and color for current display type
        const dataValue = PortalUtils.getDataValue(station, currentDisplayType);
        const formattedValue = PortalUtils.formatDataValue(dataValue, currentDisplayType);
        const dataColor = PortalUtils.getDataColor(dataValue, currentDisplayType);
        
        // Create custom marker icon with data display
        const iconClass = PortalUtils.getMarkerClass(station, currentDisplayType);
        let iconHtml = `<div class="weather-station-marker ${iconClass}" style="background-color: ${dataColor};">${formattedValue}`;
        
        // Add wind direction indicator if displaying wind speed
        if (currentDisplayType === 'wind' && station.weather && station.weather.windd) {
            const windDirection = parseFloat(station.weather.windd);
            const windSpeed = station.weather.winds ? parseFloat(station.weather.winds) : 0;
            const triangleColor = PortalUtils.getWindTriangleColor(windSpeed);
            iconHtml += `<div class="wind-direction-indicator" style="border-bottom-color: ${triangleColor}; transform: rotate(${windDirection}deg) translateY(calc(var(--marker-radius) * -1 + var(--triangle-size) * 0.577 + var(--marker-border)));"></div>`;
        }
        
        iconHtml += `</div>`;
        
        const customIcon = L.divIcon({
            html: iconHtml,
            className: 'custom-marker',
            iconSize: [40, 40],
            iconAnchor: [20, 20],
            popupAnchor: [0, -20]
        });
        
        const marker = L.marker([station.latitude, station.longitude], {
            icon: customIcon
        });
        
        // Store station data for tracking
        marker.stationName = station.name;
        marker.stationData = station;
        marker.displayType = currentDisplayType;
        
        // Handle clicks differently for air quality mode
        marker.on('click', (e) => {
            // Check if we're in air quality mode
            if (marker.displayType === 'airquality') {
                // Prevent default popup
                e.target.closePopup();
                // Trigger air quality modal through the app
                if (PortalMap.appInstance && PortalMap.appInstance.showAirQualityModal) {
                    PortalMap.appInstance.showAirQualityModal(marker.stationData);
                } else if (window.portalApp && window.portalApp.showAirQualityModal) {
                    // Fallback to global if available
                    window.portalApp.showAirQualityModal(marker.stationData);
                }
                return false; // Prevent further event propagation
            }
        }); // Arrow function preserves context
        
        // Always bind popup but it won't show in air quality mode due to click handler
        const popupContent = this.createPopupContent(station);
        marker.bindPopup(popupContent, {
            maxWidth: 320,
            className: 'weather-popup'
        });
        
        return marker;
    },

    // Update existing marker content
    updateMarkerContent(marker, station, currentDisplayType) {
        // Update stored station data and display type
        marker.stationData = station;
        marker.displayType = currentDisplayType;
        
        // Get data value and color for current display type
        const dataValue = PortalUtils.getDataValue(station, currentDisplayType);
        const formattedValue = PortalUtils.formatDataValue(dataValue, currentDisplayType);
        const dataColor = PortalUtils.getDataColor(dataValue, currentDisplayType);
        
        // Update marker icon with data display
        const iconClass = PortalUtils.getMarkerClass(station, currentDisplayType);
        let iconHtml = `<div class="weather-station-marker ${iconClass}" style="background-color: ${dataColor};">${formattedValue}`;
        
        // Add wind direction indicator if displaying wind speed
        if (currentDisplayType === 'wind' && station.weather && station.weather.windd) {
            const windDirection = parseFloat(station.weather.windd);
            const windSpeed = station.weather.winds ? parseFloat(station.weather.winds) : 0;
            const triangleColor = PortalUtils.getWindTriangleColor(windSpeed);
            iconHtml += `<div class="wind-direction-indicator" style="border-bottom-color: ${triangleColor}; transform: rotate(${windDirection}deg) translateY(calc(var(--marker-radius) * -1 + var(--triangle-size) * 0.577 + var(--marker-border)));"></div>`;
        }
        
        iconHtml += `</div>`;
        
        const customIcon = L.divIcon({
            html: iconHtml,
            className: 'custom-marker',
            iconSize: [40, 40],
            iconAnchor: [20, 20],
            popupAnchor: [0, -20]
        });
        
        marker.setIcon(customIcon);
        
        // Update popup content (won't show in air quality mode due to click handler)
        const popupContent = this.createPopupContent(station);
        marker.setPopupContent(popupContent);
    },

    // Create popup content for a station
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
                { label: 'Temperature', value: PortalUtils.formatTemperature(station.weather.otemp) },
                { label: 'Humidity', value: PortalUtils.formatHumidity(station.weather.ohum) },
                { label: 'Pressure', value: PortalUtils.formatPressure(station.weather.bar) },
                { label: 'Wind Speed', value: PortalUtils.formatWindSpeed(station.weather.winds) }
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
            const url = PortalUtils.constructStationUrl(station.website);
            
            link.href = url;
            link.target = '_blank';
            link.rel = 'noopener noreferrer';
            
            const websiteName = station.website.page_title || station.website.name;
            link.textContent = `↗ Open ${websiteName}`;
            
            websiteLink.appendChild(link);
            container.appendChild(websiteLink);
        }
        
        return container;
    },

    // Create wind rose visualization
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
        windCardinalDir.textContent = PortalUtils.getWindDirectionText(weatherData);
        
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
        const windDirection = PortalUtils.getWindDirection(weatherData);
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
    },

    // Focus map on a specific station
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
};

// Make PortalMap globally accessible
window.PortalMap = PortalMap;

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = PortalMap;
}