// Weather Portal Data Service
// Handles all API interactions and data fetching

const PortalDataService = {
    // API endpoints
    endpoints: {
        stations: '/api/stations',
        latestWeather: '/latest'
    },

    // Fetch all station data from the API
    async fetchStations() {
        try {
            const response = await fetch(this.endpoints.stations);
            if (!response.ok) {
                throw new Error('Failed to fetch station data');
            }
            
            const stationData = await response.json();
            return stationData;
        } catch (error) {
            console.error('Error fetching station data:', error);
            throw error;
        }
    },

    // Fetch weather data for a specific station
    async fetchWeatherForStation(stationName) {
        try {
            const response = await fetch(`${this.endpoints.latestWeather}?station=${stationName}`);
            if (!response.ok) {
                return { status: 'offline', weather: null };
            }
            
            const weatherData = await response.json();
            return { 
                status: 'online', 
                weather: weatherData,
                lastUpdate: new Date()
            };
        } catch (error) {
            console.error(`Error loading weather for ${stationName}:`, error);
            return { status: 'error', weather: null };
        }
    },

    // Load weather data for all stations
    async loadWeatherForAllStations(stations) {
        const weatherPromises = stations.map(async (station) => {
            const result = await this.fetchWeatherForStation(station.name);
            
            // Update station object with weather data
            station.weather = result.weather;
            station.status = result.status;
            station.lastUpdate = result.lastUpdate;
            
            return station;
        });
        
        await Promise.all(weatherPromises);
        return stations;
    },

    // Combined method to fetch all stations and their weather data
    async fetchAllStationData() {
        try {
            // First, get all stations
            const stations = await this.fetchStations();
            
            // Then, load weather data for all stations
            const stationsWithWeather = await this.loadWeatherForAllStations(stations);
            
            return stationsWithWeather;
        } catch (error) {
            console.error('Error loading station data:', error);
            throw error;
        }
    }
};

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = PortalDataService;
}