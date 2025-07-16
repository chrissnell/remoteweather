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
        calculateDataModulo
    };
})();

// Make available globally if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherUtils;
}