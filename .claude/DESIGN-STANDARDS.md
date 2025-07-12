# JavaScript Refactoring Design Standards

## Overview
This document outlines the design standards and principles used in refactoring the RemoteWeather JavaScript codebase from a monolithic structure to a modular, maintainable architecture.

## Core Principles

### 1. Incremental Refactoring
- **Never break existing functionality** - The refactored code must maintain 100% feature parity
- **Allow side-by-side testing** - New implementation should run alongside the old one
- **Create separate endpoints** - Use different routes (e.g., `/refactor`) to test new versions
- **Preserve all template variables** - Maintain compatibility with existing Go templates

### 2. Module Architecture

#### Module Types
1. **Utility Modules** - Pure functions with no side effects or DOM dependencies
2. **Service Modules** - Handle external interactions (API calls, data fetching)
3. **DOM Modules** - Centralize all DOM manipulation
4. **Component Modules** - Feature-specific logic (e.g., charts)
5. **Application Module** - Orchestrates all other modules

#### Module Structure
```javascript
const ModuleName = (function() {
    'use strict';
    
    // Private variables and functions
    
    // Public API
    return {
        publicMethod1,
        publicMethod2
    };
})();
```

### 3. Separation of Concerns

#### WeatherUtils
- Pure utility functions
- No DOM access
- No global state
- Examples: formatting, calculations, date operations

#### WeatherDataService
- All API endpoints
- Data fetching logic
- Data transformation/processing
- Error handling for network requests

#### WeatherDOM
- All DOM updates
- Element caching for performance
- Batch update operations
- No data processing logic

#### WeatherCharts
- Chart configuration
- Chart lifecycle management
- Chart-specific data processing
- Theme-aware styling

#### WeatherApp
- Application initialization
- State management
- Event listener setup
- Module orchestration

### 4. Performance Optimizations

#### DOM Caching
```javascript
const elementCache = new Map();

const getCachedElement = (elementId) => {
    if (!elementCache.has(elementId)) {
        const element = document.getElementById(elementId);
        if (element) {
            elementCache.set(elementId, element);
        }
    }
    return elementCache.get(elementId);
};
```

#### Batch Updates
- Group related DOM updates
- Use requestAnimationFrame for visual updates
- Minimize reflows and repaints

#### Parallel Data Fetching
```javascript
const [weather, snow] = await Promise.all([
    fetchLatestWeather(),
    fetchSnowData()
]);
```

### 5. Error Handling

#### Graceful Degradation
- Always provide fallback values
- Handle null/undefined cases explicitly
- Continue operation even if some data fails

#### Debug Mode
- Check for `?debug=1` in URL
- Provide detailed console logging in debug mode
- Minimal logging in production

### 6. Template Compatibility

#### Preserve Template Variables
```javascript
// Template data structure must match original
const templateData = {
    StationName: website.Name,
    PullFromDevice: primaryDevice,
    SnowEnabled: website.SnowEnabled,
    SnowDevice: website.SnowDeviceName,
    SnowBaseDistance: h.getSnowBaseDistance(website),
    PageTitle: website.PageTitle,
    AboutStationHTML: htmltemplate.HTML(website.AboutStationHTML),
};
```

#### HTML Structure
- Maintain exact same element IDs
- Keep same class names
- Preserve data attributes
- Don't use advanced template features not supported by Go templates

### 7. Configuration Management

#### Centralized Constants
```javascript
const INTERVALS = {
    LIVE_DATA: 3500,
    CHART_REFRESH: 300000,
    FORECAST_REFRESH: 4 * 60 * 60 * 1000
};
```

#### Configuration from Templates
```javascript
const config = {
    pullFromDevice: '{{ .PullFromDevice }}',
    snowEnabled: {{ .SnowEnabled }},
    snowDevice: '{{ .SnowDevice }}',
    debug: new URLSearchParams(window.location.search).get('debug') === '1'
};
```

### 8. State Management

#### Minimal Global State
- Keep state within module closures
- Pass data explicitly between modules
- Avoid global variables

#### Clear State Structure
```javascript
let state = {
    currentRange: '24h',
    currentForecastType: 'week',
    secondsSinceLastUpdate: 0,
    liveDataTimer: null,
    updateTimerInterval: null
};
```

### 9. Testing Strategy

#### Parallel Implementation
1. Create new route handler for refactored version
2. Serve refactored templates from different endpoint
3. Keep original implementation untouched
4. Compare functionality side-by-side

#### Migration Path
1. Deploy refactored version at alternate route
2. Test thoroughly in production environment
3. Gradually migrate users
4. Remove old implementation only after full validation

### 10. Code Style

#### Consistent Naming
- camelCase for variables and functions
- PascalCase for module names
- UPPER_CASE for constants
- Descriptive names over comments

#### Function Organization
1. Constants and configuration
2. Private helper functions
3. Public API methods
4. Return public interface

#### Error Messages
- Use consistent error format
- Include context in error messages
- Log errors in debug mode only

## Implementation Checklist

When refactoring JavaScript code:

- [ ] Identify all global variables and functions
- [ ] Group related functionality into modules
- [ ] Extract pure utility functions first
- [ ] Create service layer for external interactions
- [ ] Centralize DOM manipulation
- [ ] Implement proper error handling
- [ ] Add debug logging
- [ ] Maintain template compatibility
- [ ] Create parallel implementation route
- [ ] Test side-by-side with original
- [ ] Document module interfaces
- [ ] Add migration guide

## Benefits of This Approach

1. **Maintainability** - Clear module boundaries make code easier to understand and modify
2. **Testability** - Modules can be tested in isolation
3. **Performance** - Optimizations like DOM caching and batch updates
4. **Reliability** - Graceful error handling prevents cascading failures
5. **Flexibility** - Easy to add new features or modify existing ones
6. **Gradual Migration** - No "big bang" deployment required