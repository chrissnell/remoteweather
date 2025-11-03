# Weather Management Portal

The Weather Management Portal is a new feature that provides a comprehensive map-based view of all weather stations in your network. Unlike individual station websites that display data from a single weather station, the portal shows all configured stations on an interactive map with real-time weather information.

## Features

- **Interactive Map**: Built with Leaflet.js, showing all weather stations with their geographic locations
- **Real-time Weather Data**: Live weather readings including temperature, humidity, wind speed/direction, and barometric pressure
- **Compact Wind Rose**: Each station popup includes a miniaturized wind rose showing current wind direction
- **Station Status Indicators**: Visual markers indicate whether stations are online, offline, or have stale data
- **Responsive Design**: Works on desktop and mobile devices
- **Auto-refresh**: Data refreshes automatically every 5 minutes

## Configuration

To set up a Weather Management Portal, use the management interface:

1. **Access the Management Interface**: Navigate to your management server (typically on port 8081)
2. **Go to Weather Websites**: Click on the "Weather Websites" tab
3. **Add Multi-Station Portal**: Click the "+ Add Multi-Station Portal" button
4. **Configure Portal Settings**:
   - **Portal Name**: Enter a descriptive name (e.g., "Regional Weather Portal")
   - **Page Title**: Title that appears in browser tabs (e.g., "Regional Weather Portal")
   - **About Portal HTML**: Optional HTML content describing the portal
   - **Hostname**: Domain name for the portal (required for virtual hosting)
   - **TLS Configuration**: Optional certificate paths for HTTPS

The portal will automatically show all weather stations that have location data configured.

### Portal vs Regular Websites

- **Regular Weather Websites**: Display data from a single weather station
- **Multi-Station Portals**: Display all stations on an interactive map
- **Device Association**: Portals don't require device association (they show all stations automatically)

## Requirements

For weather stations to appear on the portal map, each station must have location data configured:

1. **Access Weather Stations**: Go to the "Weather Stations" tab in the management interface
2. **Edit Each Station**: Click "Edit" on each weather station
3. **Configure Location Data**:
   - **Latitude**: Required for map positioning
   - **Longitude**: Required for map positioning  
   - **Altitude**: Optional but recommended for accuracy

Stations without latitude/longitude coordinates will not appear on the portal map.

## How It Works

1. **Virtual Hosting**: The portal is served as a separate website through the REST server's virtual hosting system
2. **Station Discovery**: The portal automatically discovers all configured weather stations that have location data
3. **Data Collection**: Weather data is fetched from the existing REST API endpoints (`/latest?station=<name>`)
4. **Map Display**: Stations are plotted on the map using their configured coordinates
5. **Real-time Updates**: The portal polls for fresh data every 5 minutes

## API Endpoints

The portal uses these internal API endpoints:

- **`/api/stations`**: Returns all stations with location data (portal websites only)
- **`/latest?station=<name>`**: Returns latest weather data for a specific station

## Portal Components

### Map View
- Interactive Leaflet.js map showing all weather stations
- Color-coded markers indicating station status:
  - **Blue**: Online and current data
  - **Orange**: Stale data (older than 1 hour)
  - **Gray**: Offline
  - **Red**: Error condition

### Station Popups
Each station marker displays a popup with:
- Station name and last update timestamp
- Current temperature, humidity, pressure, and wind speed
- Compact wind rose showing wind direction
- Wind speed and cardinal direction text

### Station Sidebar
- List of all stations with quick status overview
- Click to focus map on specific station
- Shows current temperature for each station

## Styling and Customization

The portal uses CSS classes that can be customized:

- `.portal-container`: Main portal layout
- `.weather-map`: Map container styling
- `.station-sidebar`: Sidebar styling
- `.popup-windrose`: Wind rose appearance
- `.weather-station-marker`: Map marker styling

## Troubleshooting

### Stations Not Appearing
- Use the management interface to ensure devices have latitude and longitude configured
- Check that devices are enabled in the "Weather Stations" tab
- Verify the website is configured as a "Multi-Station Portal" (not a regular website)

### No Weather Data
- Confirm TimescaleDB is configured and accessible through the management interface
- Check that weather data is being collected for the stations
- Verify API endpoints are accessible

### Map Not Loading
- Check browser console for JavaScript errors
- Ensure Leaflet.js CDN is accessible
- Verify network connectivity

### Access Denied
- Confirm the website hostname matches the request in the management interface
- Check TLS certificate configuration if using HTTPS
- Verify virtual hosting is working correctly

## Management Interface Access

All portal configuration is done through the web-based management interface:

- **Default URL**: `http://your-server:8081`
- **Authentication**: Requires API token (configured during server setup)
- **Real-time Updates**: Changes take effect immediately without server restart

## Security Considerations

- Portal websites should use HTTPS in production
- Consider restricting portal access via firewall rules
- The stations API is only available to portal websites
- All weather data APIs include CORS headers for browser access 