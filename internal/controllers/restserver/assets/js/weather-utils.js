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
    
    // Get air quality level for PM1.0 (μg/m³)
    const getPM1Level = (value) => {
        if (value === null || value === undefined) return AirQualityLevels.UNKNOWN;
        
        // PM1.0 thresholds (more stringent than PM2.5)
        if (value <= 10.0) return AirQualityLevels.GOOD;
        if (value <= 25.0) return AirQualityLevels.MODERATE;
        if (value <= 50.0) return AirQualityLevels.UNHEALTHY;
        return AirQualityLevels.UNHEALTHY;
    };
    
    // Get air quality level for PM2.5 (μg/m³)
    const getPM25Level = (value) => {
        if (value === null || value === undefined) return AirQualityLevels.UNKNOWN;
        
        // EPA PM2.5 24-hour average standards
        if (value <= 12.0) return AirQualityLevels.GOOD;
        if (value <= 35.4) return AirQualityLevels.MODERATE;
        if (value <= 55.4) return AirQualityLevels.UNHEALTHY;
        if (value <= 150.4) return AirQualityLevels.UNHEALTHY;
        if (value <= 250.4) return AirQualityLevels.VERY_UNHEALTHY;
        return AirQualityLevels.HAZARDOUS;
    };
    
    // Get air quality level for PM10 (μg/m³)
    const getPM10Level = (value) => {
        if (value === null || value === undefined) return AirQualityLevels.UNKNOWN;
        
        // EPA PM10 24-hour average standards
        if (value <= 54) return AirQualityLevels.GOOD;
        if (value <= 154) return AirQualityLevels.MODERATE;
        if (value <= 254) return AirQualityLevels.UNHEALTHY;
        if (value <= 354) return AirQualityLevels.UNHEALTHY;
        if (value <= 424) return AirQualityLevels.VERY_UNHEALTHY;
        return AirQualityLevels.HAZARDOUS;
    };
    
    // Get air quality level for CO2 (ppm)
    const getCO2Level = (value) => {
        if (value === null || value === undefined) return AirQualityLevels.UNKNOWN;
        
        // Indoor air quality CO2 standards
        if (value <= 800) return AirQualityLevels.EXCELLENT;
        if (value <= 1000) return AirQualityLevels.GOOD;
        if (value <= 1500) return AirQualityLevels.FAIR;
        if (value <= 2000) return AirQualityLevels.POOR;
        if (value <= 5000) return AirQualityLevels.VERY_UNHEALTHY;
        return AirQualityLevels.DANGEROUS;
    };
    
    // Get air quality level for TVOC Index
    const getTVOCLevel = (value) => {
        if (value === null || value === undefined) return AirQualityLevels.UNKNOWN;
        
        // TVOC Index thresholds
        if (value <= 100) return AirQualityLevels.EXCELLENT;
        if (value <= 200) return AirQualityLevels.GOOD;
        if (value <= 300) return AirQualityLevels.FAIR;
        if (value <= 400) return AirQualityLevels.POOR;
        return AirQualityLevels.VERY_UNHEALTHY;
    };
    
    // Get air quality level for NOx Index
    const getNOxLevel = (value) => {
        if (value === null || value === undefined) return AirQualityLevels.UNKNOWN;
        
        // NOx Index thresholds
        if (value <= 20) return AirQualityLevels.EXCELLENT;
        if (value <= 50) return AirQualityLevels.GOOD;
        if (value <= 100) return AirQualityLevels.FAIR;
        if (value <= 200) return AirQualityLevels.POOR;
        return AirQualityLevels.VERY_UNHEALTHY;
    };
    
    // Get air quality level for AQI
    const getAQILevel = (value) => {
        if (value === null || value === undefined) return AirQualityLevels.UNKNOWN;
        
        // Standard AQI ranges
        if (value <= 50) return AirQualityLevels.GOOD;
        if (value <= 100) return AirQualityLevels.MODERATE;
        if (value <= 150) return AirQualityLevels.UNHEALTHY;
        if (value <= 200) return AirQualityLevels.UNHEALTHY;
        if (value <= 300) return AirQualityLevels.VERY_UNHEALTHY;
        return AirQualityLevels.HAZARDOUS;
    };
    
    // Universal function to get air quality level for any metric
    const getAirQualityLevel = (metricType, value) => {
        switch(metricType.toLowerCase()) {
            case 'pm1':
            case 'pm1.0':
                return getPM1Level(value);
            case 'pm25':
            case 'pm2.5':
                return getPM25Level(value);
            case 'pm10':
                return getPM10Level(value);
            case 'co2':
                return getCO2Level(value);
            case 'tvoc':
            case 'tvocindex':
                return getTVOCLevel(value);
            case 'nox':
            case 'noxindex':
                return getNOxLevel(value);
            case 'aqi':
                return getAQILevel(value);
            default:
                return AirQualityLevels.UNKNOWN;
        }
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
        getAirQualityLevel,
        getAirQualityColor,
        getAirQualityMetricColor,
        getAirQualityStatusText
    };
})();

// Make available globally if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherUtils;
}