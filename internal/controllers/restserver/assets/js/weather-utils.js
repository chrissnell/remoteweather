// Weather Dashboard Utilities Module
// This module contains pure utility functions that can be used by the main weather.js
// without any dependencies on DOM or global state

const WeatherUtils = (function() {
    'use strict';
    
    // Temperature utilities
    const formatTemperature = (temp) => {
        return temp != null ? `${Math.round(parseFloat(temp))}°F` : '--°F';
    };
    
    const formatTemperatureValue = (temp) => {
        return temp != null ? `${Math.round(parseFloat(temp))}` : '--';
    };
    
    const calculateDewPoint = (temp, humidity) => {
        if (!temp || !humidity) return null;
        const t = parseFloat(temp);
        const h = parseFloat(humidity);
        const a = 17.27;
        const b = 237.7;
        const alpha = ((a * t) / (b + t)) + Math.log(h / 100.0);
        return (b * alpha) / (a - alpha);
    };
    
    // General formatting utilities
    const formatValue = (value, decimals = 1) => {
        return value != null ? parseFloat(value).toFixed(decimals) : '--';
    };
    
    // Sky condition calculations
    const calculateSkyConditions = (current, max) => {
        // Handle null/undefined values
        if (current == null || max == null) {
            return '--';
        }
        
        const currentValue = parseFloat(current);
        const maxValue = parseFloat(max);
        
        // Handle NaN values
        if (isNaN(currentValue) || isNaN(maxValue)) {
            return '--';
        }
        
        // If potential solar is very low, it's night
        if (maxValue < 10) return 'Night';
        
        // Handle zero max value to avoid division by zero
        if (maxValue === 0) return 'Night';
        
        // Calculate percentage
        const percentage = (currentValue / maxValue) * 100;
        
        if (percentage >= 80) return 'Sunny';
        if (percentage >= 40) return 'Partly Cloudy';
        return 'Cloudy';
    };
    
    // Battery status calculation
    const getBatteryStatus = (voltage) => {
        if (!voltage) return '--';
        const v = parseFloat(voltage);
        if (v >= 12.6) return 'Good';
        if (v >= 12.0) return 'Fair';
        return 'Low';
    };
    
    // CSS variable utility
    const getCSSVariable = (name) => {
        return getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    };
    
    // Async fetch with timeout
    const fetchWithTimeout = async (url, timeout = 10000) => {
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), timeout);
        
        try {
            const response = await fetch(url, { signal: controller.signal });
            clearTimeout(timeoutId);
            if (!response.ok) throw new Error(`HTTP error: ${response.status}`);
            return response.json();
        } catch (error) {
            clearTimeout(timeoutId);
            console.error(`Failed to fetch ${url}:`, error);
            return null;
        }
    };
    
    // Date/time formatting utilities
    const formatLastUpdated = (dateString) => {
        if (!window.moment) return dateString;
        return moment(dateString, "YYYY-MM-DD HH:mm:ss.SSSSSS-ZZ").format("h:mm A, DD MMM YYYY");
    };
    
    const getDayName = (dayIndex) => {
        const dayNames = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
        return dayNames[dayIndex] || "";
    };
    
    // Weather code parsing utilities
    const isSnowWeather = (weatherCode) => {
        const snowRegex = /.*:.*:(S|SW|SI|RS|WM|BS)$/;
        return snowRegex.test(weatherCode);
    };
    
    const isRainWeather = (weatherCode) => {
        const rainRegex = /.*:.*:(R|RW|T|TO|UP|ZR|L|BY|ZY|ZR)$/;
        return rainRegex.test(weatherCode);
    };
    
    // Chart data processing utilities
    const calculateDataModulo = (dataLength, maxPoints = 50) => {
        // Detect if viewing on mobile device
        const isMobile = window.matchMedia && window.matchMedia('(max-width: 768px)').matches;
        
        // Reduce maxPoints to 1/3 for mobile devices to prevent crowding
        const adjustedMaxPoints = isMobile ? Math.floor(maxPoints / 3) : maxPoints;
        
        return Math.max(1, Math.floor(dataLength / adjustedMaxPoints));
    };
    
    // Air Quality System - Centralized definitions
    const AirQualityLevels = {
        EXCELLENT: 'EXCELLENT',
        GOOD: 'GOOD',
        FAIR: 'FAIR',
        MODERATE: 'MODERATE',
        POOR: 'POOR',
        UNHEALTHY: 'UNHEALTHY',
        VERY_UNHEALTHY: 'VERY_UNHEALTHY',
        HAZARDOUS: 'HAZARDOUS',
        DANGEROUS: 'DANGEROUS',
        UNKNOWN: 'UNKNOWN'
    };
    
    // Centralized color scheme for air quality levels (using solarized colors)
    const AirQualityColors = {
        [AirQualityLevels.EXCELLENT]: '#859900',  // Green
        [AirQualityLevels.GOOD]: '#2980b9',       // Blue
        [AirQualityLevels.FAIR]: '#b58900',       // Yellow
        [AirQualityLevels.MODERATE]: '#b58900',   // Yellow
        [AirQualityLevels.POOR]: '#cb4b16',       // Orange
        [AirQualityLevels.UNHEALTHY]: '#dc322f',  // Red
        [AirQualityLevels.VERY_UNHEALTHY]: '#d33682', // Magenta
        [AirQualityLevels.HAZARDOUS]: '#6c71c4',  // Violet
        [AirQualityLevels.DANGEROUS]: '#d33682',  // Magenta
        [AirQualityLevels.UNKNOWN]: '#7f8c8d'     // Gray
    };
    
    // Define thresholds as data structures - single source of truth
    const AirQualityThresholds = {
        pm1: [
            { max: 10.0, level: AirQualityLevels.GOOD },
            { max: 25.0, level: AirQualityLevels.MODERATE },
            { max: 50.0, level: AirQualityLevels.UNHEALTHY },
            { max: Infinity, level: AirQualityLevels.UNHEALTHY }
        ],
        pm25: [
            { max: 12.0, level: AirQualityLevels.GOOD },
            { max: 35.4, level: AirQualityLevels.MODERATE },
            { max: 55.4, level: AirQualityLevels.UNHEALTHY },
            { max: 150.4, level: AirQualityLevels.UNHEALTHY },
            { max: 250.4, level: AirQualityLevels.VERY_UNHEALTHY },
            { max: Infinity, level: AirQualityLevels.HAZARDOUS }
        ],
        pm10: [
            { max: 54, level: AirQualityLevels.GOOD },
            { max: 154, level: AirQualityLevels.MODERATE },
            { max: 254, level: AirQualityLevels.UNHEALTHY },
            { max: 354, level: AirQualityLevels.UNHEALTHY },
            { max: 424, level: AirQualityLevels.VERY_UNHEALTHY },
            { max: Infinity, level: AirQualityLevels.HAZARDOUS }
        ],
        co2: [
            { max: 800, level: AirQualityLevels.EXCELLENT },
            { max: 1000, level: AirQualityLevels.GOOD },
            { max: 1500, level: AirQualityLevels.FAIR },
            { max: 2000, level: AirQualityLevels.POOR },
            { max: 5000, level: AirQualityLevels.VERY_UNHEALTHY },
            { max: Infinity, level: AirQualityLevels.DANGEROUS }
        ],
        tvocindex: [
            { max: 100, level: AirQualityLevels.EXCELLENT },
            { max: 200, level: AirQualityLevels.GOOD },
            { max: 300, level: AirQualityLevels.FAIR },
            { max: 400, level: AirQualityLevels.POOR },
            { max: Infinity, level: AirQualityLevels.VERY_UNHEALTHY }
        ],
        noxindex: [
            { max: 20, level: AirQualityLevels.EXCELLENT },
            { max: 50, level: AirQualityLevels.GOOD },
            { max: 100, level: AirQualityLevels.FAIR },
            { max: 200, level: AirQualityLevels.POOR },
            { max: Infinity, level: AirQualityLevels.VERY_UNHEALTHY }
        ],
        aqi: [
            { max: 50, level: AirQualityLevels.GOOD },
            { max: 100, level: AirQualityLevels.MODERATE },
            { max: 150, level: AirQualityLevels.UNHEALTHY },
            { max: 200, level: AirQualityLevels.UNHEALTHY },
            { max: 300, level: AirQualityLevels.VERY_UNHEALTHY },
            { max: Infinity, level: AirQualityLevels.HAZARDOUS }
        ]
    };
    
    // Get air quality level for any metric using the threshold data
    const getAirQualityLevel = (metricType, value) => {
        if (value === null || value === undefined) return AirQualityLevels.UNKNOWN;
        
        // Normalize metric type
        let normalizedType = metricType.toLowerCase();
        if (normalizedType === 'pm1.0') normalizedType = 'pm1';
        if (normalizedType === 'pm2.5') normalizedType = 'pm25';
        if (normalizedType === 'tvoc') normalizedType = 'tvocindex';
        if (normalizedType === 'nox') normalizedType = 'noxindex';
        
        // Get thresholds for this metric
        const thresholds = AirQualityThresholds[normalizedType];
        if (!thresholds) return AirQualityLevels.UNKNOWN;
        
        // Find the appropriate level based on value
        for (const threshold of thresholds) {
            if (value <= threshold.max) {
                return threshold.level;
            }
        }
        
        // Should never reach here, but return UNKNOWN as fallback
        return AirQualityLevels.UNKNOWN;
    };
    
    // Get color for air quality level
    const getAirQualityColor = (level) => {
        return AirQualityColors[level] || AirQualityColors[AirQualityLevels.UNKNOWN];
    };
    
    // Get color for a specific metric and value
    const getAirQualityMetricColor = (metricType, value) => {
        const level = getAirQualityLevel(metricType, value);
        return getAirQualityColor(level);
    };
    
    // Get human-readable status text for air quality level
    const getAirQualityStatusText = (level) => {
        const statusMap = {
            [AirQualityLevels.EXCELLENT]: 'Excellent',
            [AirQualityLevels.GOOD]: 'Good',
            [AirQualityLevels.FAIR]: 'Fair',
            [AirQualityLevels.MODERATE]: 'Moderate',
            [AirQualityLevels.POOR]: 'Poor',
            [AirQualityLevels.UNHEALTHY]: 'Unhealthy',
            [AirQualityLevels.VERY_UNHEALTHY]: 'Very Unhealthy',
            [AirQualityLevels.HAZARDOUS]: 'Hazardous',
            [AirQualityLevels.DANGEROUS]: 'Dangerous',
            [AirQualityLevels.UNKNOWN]: '--'
        };
        return statusMap[level] || '--';
    };
    
    // Get thresholds for a specific air quality metric for chart zones
    const getAirQualityThresholds = (metricType) => {
        // Normalize metric type
        let normalizedType = metricType.toLowerCase();
        if (normalizedType === 'pm1.0') normalizedType = 'pm1';
        if (normalizedType === 'pm2.5') normalizedType = 'pm25';
        if (normalizedType === 'tvoc') normalizedType = 'tvocindex';
        if (normalizedType === 'nox') normalizedType = 'noxindex';
        
        return AirQualityThresholds[normalizedType] || [];
    };
    
    // Public API
    return {
        // Temperature
        formatTemperature,
        formatTemperatureValue,
        calculateDewPoint,
        
        // Formatting
        formatValue,
        
        // Calculations
        calculateSkyConditions,
        getBatteryStatus,
        
        // CSS
        getCSSVariable,
        
        // Async utilities
        fetchWithTimeout,
        
        // Date/time
        formatLastUpdated,
        getDayName,
        
        // Weather codes
        isSnowWeather,
        isRainWeather,
        
        // Data processing
        calculateDataModulo,
        
        // Air Quality System
        AirQualityLevels,
        AirQualityColors,
        AirQualityThresholds,
        getAirQualityLevel,
        getAirQualityColor,
        getAirQualityMetricColor,
        getAirQualityStatusText,
        getAirQualityThresholds
    };
})();

// Make available globally if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherUtils;
}