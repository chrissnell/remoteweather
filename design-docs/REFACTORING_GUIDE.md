# Weather Dashboard Refactoring Guide

## Overview

The weather dashboard JavaScript has been refactored from a monolithic `weather.js.tmpl` file into a modular, component-based architecture. This guide explains the new structure and how to migrate.

## New Module Structure

### 1. **weather-utils.js** - Pure Utility Functions
- Temperature formatting and calculations
- General value formatting  
- Sky condition calculations
- Battery status determination
- CSS variable access
- Async fetch with timeout
- Date/time formatting
- Weather code parsing

### 2. **weather-charts.js** - Chart Management
- Chart creation and configuration
- Chart type definitions
- Data processing for different chart types
- Chart lifecycle management (create/destroy)
- Theme-aware chart styling

### 3. **weather-data-service.js** - API and Data Layer
- All API endpoint definitions
- Data fetching methods
- Data processing and transformation
- Combined fetch operations for efficiency
- Error handling for network requests

### 4. **weather-dom.js** - DOM Manipulation
- Centralized DOM updates
- Element caching for performance
- Batch update operations
- UI state management (tabs, indicators)
- Specialized update methods for each data type

### 5. **weather-app.js.tmpl** - Main Application
- Application initialization
- State management
- Event listener setup
- Timer management
- Orchestrates all modules

## Migration Steps

### Step 1: Test Current Implementation
Before making any changes, ensure your current `weather.js.tmpl` is working correctly.

### Step 2: Deploy New Modules
1. Copy the new module files to your assets/js directory:
   - weather-utils.js
   - weather-charts.js
   - weather-data-service.js
   - weather-dom.js
   - weather-app.js (generated from weather-app.js.tmpl)

### Step 3: Update HTML Template
You have two options:

**Option A: Side-by-side Testing**
- Use `weather-refactored.html.tmpl` alongside your existing `weather.html.tmpl`
- This allows you to test the new implementation without affecting the current one

**Option B: Direct Migration**
- Update your existing `weather.html.tmpl` to include the new modules:
```html
<!-- Replace this -->
<script src="js/weather.js"></script>

<!-- With these -->
<script src="js/weather-utils.js"></script>
<script src="js/weather-charts.js"></script>
<script src="js/weather-data-service.js"></script>
<script src="js/weather-dom.js"></script>
<script src="js/weather-app.js"></script>
```

### Step 4: Template Variable Compatibility
The new modules use the same template variables as the original:
- `{{ .PullFromDevice }}`
- `{{ .SnowEnabled }}`
- `{{ .SnowDevice }}`
- `{{ .StationName }}`

No changes needed to your Go template processing.

### Step 5: Testing Checklist
- [ ] Live data updates every 3.5 seconds
- [ ] All weather metrics display correctly
- [ ] Charts load and display data
- [ ] Chart range switching works (24h, 72h, 7d, 30d, 1y)
- [ ] Forecast tabs switch correctly
- [ ] Theme toggle works and charts update
- [ ] Snow data displays (if enabled)
- [ ] Battery status shows/hides appropriately
- [ ] No console errors

## Key Improvements

### 1. **Maintainability**
- Clear separation of concerns
- Each module has a single responsibility
- Easier to debug and modify

### 2. **Performance**
- DOM element caching reduces lookups
- Batch updates minimize reflows
- Parallel data fetching

### 3. **Testability**
- Pure functions in utilities
- Mockable data service
- Isolated DOM operations

### 4. **Extensibility**
- Easy to add new chart types
- Simple to add new weather metrics
- Modular structure allows for gradual enhancements

## Customization Points

### Adding a New Chart Type
1. Add configuration to `chartTypeConfigs` in weather-charts.js
2. Add data processing case in `processChartData`
3. Update HTML template with new chart containers

### Adding a New Weather Metric
1. Add processing in `processLiveWeatherData` in weather-data-service.js
2. Add DOM update in `updateLiveWeather` in weather-dom.js
3. Add HTML element with appropriate ID

### Modifying Data Refresh Intervals
Update the `INTERVALS` object in weather-app.js:
```javascript
const INTERVALS = {
    LIVE_DATA: 3500,        // milliseconds
    CHART_REFRESH: 300000,  // milliseconds
    FORECAST_REFRESH: 4 * 60 * 60 * 1000  // milliseconds
};
```

## Rollback Plan

If you need to rollback:
1. Simply revert the HTML template to use the original weather.js
2. The new modules don't affect the old code
3. Both can coexist during transition

## Debugging Tips

1. **Enable Debug Mode**: Add `?debug=1` to URL
2. **Check Module Loading**: Verify all modules load in browser network tab
3. **Console Logging**: Each module logs errors when debug=1
4. **Check Template Processing**: View page source to ensure template variables are replaced

## Common Issues

### Charts Not Displaying
- Check that Highcharts library is loaded before modules
- Verify element IDs match between HTML and JavaScript
- Check browser console for errors

### Data Not Updating
- Verify API endpoints are accessible
- Check network tab for failed requests
- Ensure template variables are properly set

### Theme Not Applying to Charts
- Ensure CSS variables are defined in stylesheets
- Check that `refreshChartsForTheme` is called after theme change

## Future Enhancements

The modular structure makes it easy to:
- Add WebSocket support for real-time updates
- Implement data caching and offline support
- Add new visualization types
- Create a mobile-specific interface
- Add data export functionality

## Support

For issues or questions:
1. Check browser console for errors
2. Verify all modules are loaded
3. Compare with original weather.js behavior
4. Test with debug mode enabled