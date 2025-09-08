// Weather Dashboard Chart Module
// This module handles all chart-related functionality

const WeatherCharts = (function() {
    'use strict';
    
    // Chart registry to track created charts
    const chartRegistry = new Map();
    
    // Track all charts by time range for synchronization
    const chartsByRange = new Map();
    
    // Helper function to get air quality chart color based on latest value
    const getAirQualityChartColor = (chartType, data) => {
        // Get the most recent value to determine the color
        if (!data || data.length === 0) return WeatherUtils.getCSSVariable('--chart-series-color');
        
        const latestValue = data[data.length - 1][1];
        return WeatherUtils.getAirQualityMetricColor(chartType, latestValue);
    };
    
    // Helper function to create color zones for air quality charts using centralized thresholds
    const getAirQualityZones = (chartType) => {
        const thresholds = WeatherUtils.getAirQualityThresholds(chartType);
        if (!thresholds || thresholds.length === 0) return null;
        
        const zones = [];
        
        // Convert thresholds to Highcharts zones format
        // Each zone defines the color up to the threshold value
        for (const threshold of thresholds) {
            const color = WeatherUtils.getAirQualityColor(threshold.level);
            
            if (threshold.max === Infinity) {
                // Last zone - no value limit
                zones.push({ color: color });
            } else {
                // Zone with upper limit
                zones.push({ 
                    value: threshold.max, 
                    color: color 
                });
            }
        }
        
        return zones;
    };
    
    // Default chart options factory
    const getDefaultChartOptions = () => ({
        chart: {
            backgroundColor: WeatherUtils.getCSSVariable('--chart-bg'),
            style: { 
                fontFamily: 'Inconsolata, Roboto, sans-serif',
                color: WeatherUtils.getCSSVariable('--chart-text')
            }
        },
        credits: { enabled: false },
        time: { useUTC: false },
        xAxis: {
            type: 'datetime',
            dateTimeLabelFormats: { hour: '%l %p', minute: '%I:%M %p' },
            lineColor: WeatherUtils.getCSSVariable('--chart-grid'),
            tickColor: WeatherUtils.getCSSVariable('--chart-grid'),
            labels: { style: { color: WeatherUtils.getCSSVariable('--chart-text') } },
            crosshair: {
                color: WeatherUtils.getCSSVariable('--chart-grid'),
                width: 1,
                dashStyle: 'Dot'
            }
        },
        tooltip: {
            backgroundColor: WeatherUtils.getCSSVariable('--chart-tooltip-bg'),
            borderColor: WeatherUtils.getCSSVariable('--chart-tooltip-border'),
            style: { color: WeatherUtils.getCSSVariable('--chart-text') }
        }
    });
    
    // Chart type configurations
    const chartTypeConfigs = {
        temperature: {
            yAxisLabel: "Degrees F",
            chartType: "spline",
            tooltipDecimals: 1,
            unit: "°F"
        },
        humidity: {
            yAxisLabel: "Percent",
            chartType: "spline",
            tooltipDecimals: 1,
            unit: "%"
        },
        snowdepth: {
            yAxisLabel: "inches",
            chartType: "spline",
            tooltipDecimals: 2,
            unit: " in"
        },
        barometer: {
            yAxisLabel: "inches Hg",
            chartType: "spline",
            tooltipDecimals: 2,
            unit: " inHg"
        },
        windspeed: {
            yAxisLabel: "MPH",
            chartType: "spline",
            tooltipDecimals: 1,
            unit: " MPH"
        },
        winddirection: {
            yAxisLabel: "",
            chartType: "vector",
            tooltipFormat: {
                pointFormat: '<span style="color:{point.color}">●</span> {point.x:%e %b, %l:%M %p}<br/>Speed: <b>{point.length:.1f} MPH</b><br/>Direction: <b>{point.direction:.0f}°</b>'
            }
        },
        rainfall: {
            yAxisLabel: "Inches",
            chartType: "column",
            tooltipDecimals: 2,
            unit: " in"
        },
        solarwatts: {
            yAxisLabel: "Watts/m²",
            chartType: "spline",
            tooltipDecimals: 1,
            unit: " W/m²",
            additionalSeries: [{
                name: "Maximum Potential Solar Radiation",
                data: [],
                color: 'rgb(255, 81, 0)'
            }]
        },
        voltage: {
            yAxisLabel: "Volts",
            chartType: "spline",
            tooltipDecimals: 2,
            unit: " V"
        },
        pm25: {
            displayName: "PM2.5",
            yAxisLabel: "µg/m³",
            chartType: "spline",
            tooltipDecimals: 1,
            unit: " µg/m³",
            plotLines: [
                { value: 12, color: '#2aa22a', width: 1, dashStyle: 'dash', label: { text: 'Good', style: { color: '#2aa22a' } } },
                { value: 35, color: '#ffbf00', width: 1, dashStyle: 'dash', label: { text: 'Moderate', style: { color: '#ffbf00' } } },
                { value: 55, color: '#ff7e00', width: 1, dashStyle: 'dash', label: { text: 'Unhealthy', style: { color: '#ff7e00' } } }
            ]
        },
        pm10: {
            displayName: "PM10",
            yAxisLabel: "µg/m³",
            chartType: "spline",
            tooltipDecimals: 1,
            unit: " µg/m³",
            plotLines: [
                { value: 54, color: '#2aa22a', width: 1, dashStyle: 'dash', label: { text: 'Good', style: { color: '#2aa22a' } } },
                { value: 154, color: '#ffbf00', width: 1, dashStyle: 'dash', label: { text: 'Moderate', style: { color: '#ffbf00' } } }
            ]
        },
        co2: {
            displayName: "CO₂",
            yAxisLabel: "ppm",
            chartType: "spline",
            tooltipDecimals: 0,
            unit: " ppm",
            plotLines: [
                { value: 800, color: '#2aa22a', width: 1, dashStyle: 'dash', label: { text: 'Good', style: { color: '#2aa22a' } } },
                { value: 1000, color: '#ffbf00', width: 1, dashStyle: 'dash', label: { text: 'Fair', style: { color: '#ffbf00' } } },
                { value: 1500, color: '#ff7e00', width: 1, dashStyle: 'dash', label: { text: 'Poor', style: { color: '#ff7e00' } } }
            ]
        },
        tvocindex: {
            displayName: "TVOC Index",
            yAxisLabel: "Index",
            chartType: "spline",
            tooltipDecimals: 0,
            unit: "",
            plotLines: [
                { value: 100, color: '#2aa22a', width: 1, dashStyle: 'dash', label: { text: 'Excellent', style: { color: '#2aa22a' } } },
                { value: 200, color: '#ffbf00', width: 1, dashStyle: 'dash', label: { text: 'Good', style: { color: '#ffbf00' } } },
                { value: 300, color: '#ff7e00', width: 1, dashStyle: 'dash', label: { text: 'Fair', style: { color: '#ff7e00' } } },
                { value: 400, color: '#ff0000', width: 1, dashStyle: 'dash', label: { text: 'Poor', style: { color: '#ff0000' } } }
            ]
        },
        noxindex: {
            displayName: "NOx Index",
            yAxisLabel: "Index",
            chartType: "spline",
            tooltipDecimals: 0,
            unit: "",
            plotLines: [
                { value: 100, color: '#2aa22a', width: 1, dashStyle: 'dash', label: { text: 'Excellent', style: { color: '#2aa22a' } } },
                { value: 200, color: '#ffbf00', width: 1, dashStyle: 'dash', label: { text: 'Good', style: { color: '#ffbf00' } } },
                { value: 300, color: '#ff7e00', width: 1, dashStyle: 'dash', label: { text: 'Fair', style: { color: '#ff7e00' } } },
                { value: 400, color: '#ff0000', width: 1, dashStyle: 'dash', label: { text: 'Poor', style: { color: '#ff0000' } } }
            ]
        }
    };
    
    // Setup synchronized tooltips (keeping the old name for compatibility)
    const setupSynchronizedTooltips = (range) => {
        if (!range) return;
        
        const container = document.getElementById(`charts-${range}`);
        if (!container || container.hasAttribute('data-sync-setup')) return;
        
        container.setAttribute('data-sync-setup', 'true');
        
        // Track mouse position for crosshair sync
        let currentEvent = null;
        
        container.addEventListener('mousemove', function(e) {
            currentEvent = e;
            const charts = chartsByRange.get(range) || [];
            
            charts.forEach(chart => {
                if (chart && chart.pointer) {
                    const event = chart.pointer.normalize(e);
                    
                    // Just draw crosshair, don't touch tooltips
                    if (chart.xAxis && chart.xAxis[0]) {
                        // Find the closest point for crosshair positioning
                        if (chart.series && chart.series[0]) {
                            const point = chart.series[0].searchPoint(event, true);
                            if (point) {
                                chart.xAxis[0].drawCrosshair(event, point);
                            }
                        }
                    }
                }
            });
        });
        
        container.addEventListener('mouseleave', function() {
            const charts = chartsByRange.get(range) || [];
            
            charts.forEach(chart => {
                // Just hide crosshair, let Highcharts handle its own tooltips
                if (chart && chart.xAxis && chart.xAxis[0]) {
                    chart.xAxis[0].hideCrosshair();
                }
            });
        });
    };
    
    // Create a single chart
    const createChart = (chartName, targetDiv, data, title, customOptions = {}) => {
        const config = chartTypeConfigs[chartName] || {};
        const { displayName, yAxisLabel, chartType = 'spline', tooltipFormat, additionalSeries = [] } = { ...config, ...customOptions };
        
        // Use display name if available, otherwise use title
        const seriesName = displayName || title;
        
        // Extract range from targetDiv for synchronization
        const rangeMatch = targetDiv.match(/(24h|72h|7d|30d|1y)$/);
        const range = rangeMatch ? rangeMatch[1] : null;
        
        const baseOptions = getDefaultChartOptions();
        
        const chartConfig = {
            ...baseOptions,
            chart: {
                ...baseOptions.chart,
                type: chartType,
                renderTo: targetDiv
            },
            title: { 
                text: null  // Remove duplicate title
            },
            yAxis: chartType === 'vector' ? { visible: false } : { 
                title: { 
                    text: yAxisLabel,
                    style: { color: WeatherUtils.getCSSVariable('--chart-text') }
                },
                gridLineColor: WeatherUtils.getCSSVariable('--chart-grid'),
                labels: { style: { color: WeatherUtils.getCSSVariable('--chart-text') } }
            },
            legend: { 
                enabled: additionalSeries.length > 0,
                align: 'center',
                verticalAlign: 'bottom',
                layout: 'horizontal',
                itemStyle: { 
                    color: WeatherUtils.getCSSVariable('--chart-text'),
                    fontSize: '12px'
                },
                itemHoverStyle: {
                    color: WeatherUtils.getCSSVariable('--chart-series-color')
                },
                backgroundColor: 'transparent',
                borderWidth: 0,
                y: 10
            },
            tooltip: {
                ...baseOptions.tooltip,
                valueDecimals: config.tooltipDecimals || 2,
                valueSuffix: config.unit || '',
                ...tooltipFormat,
                // Custom formatter for air quality charts to show status
                formatter: ['pm25', 'pm10', 'co2', 'tvocindex', 'noxindex'].includes(chartName) ? function() {
                        const value = this.y;
                        const level = WeatherUtils.getAirQualityLevel(chartName, value);
                        const status = WeatherUtils.getAirQualityStatusText(level);
                        const color = WeatherUtils.getAirQualityColor(level);
                        const decimals = config.tooltipDecimals || 2;
                        const unit = config.unit || '';
                        
                        return `<b>${Highcharts.dateFormat('%A, %b %e, %l:%M %p', this.x)}</b><br/>` +
                               `<span style="color:${this.color}">\u25CF</span> ${seriesName}: <b>${value.toFixed(decimals)}${unit}</b><br/>` +
                               `<span style="color:${color}">\u25CF</span> Status: <b style="color:${color}">${status}</b>`;
                } : undefined
            },
            series: [
                {
                    name: seriesName,
                    data: data,
                    visible: true,
                    color: ['pm25', 'pm10', 'co2', 'tvocindex', 'noxindex'].includes(chartName) 
                        ? getAirQualityChartColor(chartName, data)
                        : WeatherUtils.getCSSVariable('--chart-series-color'),
                    zones: ['pm25', 'pm10', 'co2', 'tvocindex', 'noxindex'].includes(chartName)
                        ? getAirQualityZones(chartName)
                        : undefined,
                    zoneAxis: 'y',
                    marker: {
                        enabled: false,
                        states: {
                            hover: {
                                enabled: true,
                                radius: 3
                            }
                        }
                    }
                },
                ...additionalSeries.map(series => ({
                    ...series,
                    visible: true,
                    color: series.color || WeatherUtils.getCSSVariable('--chart-series-color-alt'),
                    dashStyle: series.dashStyle || 'Solid',
                    marker: {
                        enabled: false,
                        states: {
                            hover: {
                                enabled: true,
                                radius: 3
                            }
                        }
                    }
                }))
            ]
        };
        
        const chart = new Highcharts.Chart(chartConfig);
        
        // Register the chart
        const chartKey = `${chartName}_${targetDiv}`;
        if (chartRegistry.has(chartKey)) {
            chartRegistry.get(chartKey).destroy();
        }
        chartRegistry.set(chartKey, chart);
        
        // Track charts by range for synchronization
        if (range) {
            if (!chartsByRange.has(range)) {
                chartsByRange.set(range, []);
            }
            chartsByRange.get(range).push(chart);
        }
        
        return chart;
    };
    
    // Process raw data for charts
    const processChartData = (rawData, chartType, options = {}) => {
        if (!rawData || !Array.isArray(rawData)) return [];
        
        const { snowData, modulo } = options;
        const dataModulo = modulo || WeatherUtils.calculateDataModulo(rawData.length);
        
        switch (chartType) {
            case 'temperature':
                return rawData.map(item => [item.ts, item.otemp]);
                
            case 'humidity':
                return rawData.map(item => [item.ts, item.outhumidity]);
                
            case 'barometer':
                return rawData.map(item => [item.ts, item.bar]);
                
            case 'windspeed':
                return rawData.map(item => [item.ts, item.winds]);
                
            case 'winddirection':
                return rawData
                    .filter((_, i) => i % dataModulo === 0 || rawData.length < 50)
                    .map(item => [item.ts, 0, item.winds, item.windd]);
                
            case 'rainfall':
                return rawData.map(item => [item.ts, item.period_rain]);
                
            case 'solarwatts':
                return rawData.map(item => [item.ts, item.solarwatts]);
                
            case 'voltage':
                return rawData.map(item => [item.ts, item.stationbatteryvoltage]);
                
            case 'snowdepth':
                return snowData ? snowData.map(item => [item.ts, item.snowdepth]) : [];
                
            case 'pm25':
                return rawData.map(item => [item.ts, item.pm25]);
                
            case 'pm10':
                return rawData.map(item => [item.ts, item.extrafloat2]); // PM10 stored in extrafloat2
                
            case 'co2':
                return rawData.map(item => [item.ts, item.co2]);
                
            case 'tvocindex':
                return rawData.map(item => [item.ts, item.extrafloat3]); // TVOC Index stored in extrafloat3
                
            case 'noxindex':
                return rawData.map(item => [item.ts, item.extrafloat4]); // NOX Index stored in extrafloat4
                
            default:
                return [];
        }
    };
    
    // Get additional series data (for charts like solar that have multiple series)
    const getAdditionalSeriesData = (rawData, chartType) => {
        if (chartType === 'temperature' && rawData) {
            const series = [];
            
            // Add feels like temperature (heat index or wind chill)
            const feelsLikeData = rawData.map(item => {
                const feelsLike = item.heatidx || item.windch || item.otemp;
                return [item.ts, feelsLike];
            });
            series.push({
                name: "Feels Like",
                data: feelsLikeData,
                color: 'rgb(255, 127, 80)',  // Coral color
                dashStyle: 'ShortDash'
            });
            
            // Add dewpoint
            const dewpointData = rawData.map(item => {
                const dewpoint = WeatherUtils.calculateDewPoint(item.otemp, item.outhumidity);
                return [item.ts, dewpoint];
            });
            series.push({
                name: "Dewpoint",
                data: dewpointData,
                color: 'rgb(135, 206, 250)',  // Light sky blue
                dashStyle: 'ShortDot'
            });
            
            return series;
        }
        
        if (chartType === 'solarwatts' && rawData) {
            return [{
                name: "Maximum Potential Solar Radiation",
                data: rawData.map(item => [item.ts, item.potentialsolarwatts])
            }];
        }
        return [];
    };
    
    // Destroy all charts
    const destroyAllCharts = () => {
        chartRegistry.forEach(chart => {
            if (chart && chart.destroy) {
                chart.destroy();
            }
        });
        chartRegistry.clear();
        chartsByRange.clear();
    };
    
    // Clear charts for a specific range
    const clearChartsForRange = (range) => {
        if (chartsByRange.has(range)) {
            chartsByRange.set(range, []);
        }
    };
    
    // Destroy specific chart
    const destroyChart = (chartKey) => {
        if (chartRegistry.has(chartKey)) {
            const chart = chartRegistry.get(chartKey);
            if (chart && chart.destroy) {
                chart.destroy();
            }
            chartRegistry.delete(chartKey);
        }
    };
    
    // Get chart configuration
    const getChartConfig = (chartName) => {
        return chartTypeConfigs[chartName] || null;
    };
    
    // Public API
    return {
        createChart,
        processChartData,
        getAdditionalSeriesData,
        destroyAllCharts,
        destroyChart,
        clearChartsForRange,
        setupSynchronizedTooltips,
        getChartConfig,
        chartTypeConfigs
    };
})();

// Make available globally if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherCharts;
}