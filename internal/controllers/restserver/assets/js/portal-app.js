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
        PortalMap.initializeMap();
        
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
            PortalMap.updateMapMarkers(this.stations, this.currentDisplayType);
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
        PortalMap.focusOnStation(station);
    }
    
    // Setup event listeners
    setupEventListeners() {
        // Setup data display button handlers
        PortalDOM.setupDataDisplayButtons((dataType) => {
            this.currentDisplayType = dataType;
            // Update all markers with new display type
            PortalMap.updateMapMarkers(this.stations, this.currentDisplayType);
        });
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