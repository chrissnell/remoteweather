// Weather Dashboard Chart Module
// This module handles all chart-related functionality

const WeatherCharts = (function() {
    'use strict';
    
    // Chart registry to track created charts
    const chartRegistry = new Map();
    
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
            labels: { style: { color: WeatherUtils.getCSSVariable('--chart-text') } }
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
            tooltipDecimals: 1
        },
        humidity: {
            yAxisLabel: "Percent",
            chartType: "spline",
            tooltipDecimals: 1
        },
        snowdepth: {
            yAxisLabel: "inches",
            chartType: "spline",
            tooltipDecimals: 2
        },
        barometer: {
            yAxisLabel: "inches Hg",
            chartType: "spline",
            tooltipDecimals: 2
        },
        windspeed: {
            yAxisLabel: "MPH",
            chartType: "spline",
            tooltipDecimals: 1
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
            tooltipDecimals: 2
        },
        solarwatts: {
            yAxisLabel: "Watts/m²",
            chartType: "spline",
            tooltipDecimals: 1,
            additionalSeries: [{
                name: "Maximum Potential Solar Radiation",
                data: [],
                color: 'rgb(255, 81, 0)'
            }]
        },
        voltage: {
            yAxisLabel: "Volts",
            chartType: "spline",
            tooltipDecimals: 2
        }
    };
    
    // Create a single chart
    const createChart = (chartName, targetDiv, data, title, customOptions = {}) => {
        const config = chartTypeConfigs[chartName] || {};
        const { yAxisTitle, chartType = 'spline', tooltipFormat, additionalSeries = [] } = { ...config, ...customOptions };
        
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
                    text: yAxisTitle,
                    style: { color: WeatherUtils.getCSSVariable('--chart-text') }
                },
                gridLineColor: WeatherUtils.getCSSVariable('--chart-grid'),
                labels: { style: { color: WeatherUtils.getCSSVariable('--chart-text') } }
            },
            legend: { 
                enabled: additionalSeries.length > 0,
                itemStyle: { color: WeatherUtils.getCSSVariable('--chart-text') }
            },
            tooltip: {
                ...baseOptions.tooltip,
                ...(tooltipFormat || { valueDecimals: config.tooltipDecimals || 2 })
            },
            series: [
                {
                    name: title,
                    data: data,
                    color: WeatherUtils.getCSSVariable('--chart-series-color'),
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
                    color: series.color || WeatherUtils.getCSSVariable('--chart-series-color-alt')
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
                    .filter((item, i) => i % dataModulo === 0 || rawData.length < 50)
                    .map(item => [item.ts, 0, item.winds, item.windd]);
                
            case 'rainfall':
                return rawData.map(item => [item.ts, item.period_rain]);
                
            case 'solarwatts':
                return rawData.map(item => [item.ts, item.solarwatts]);
                
            case 'voltage':
                return rawData.map(item => [item.ts, item.stationbatteryvoltage]);
                
            case 'snowdepth':
                return snowData ? snowData.map(item => [item.ts, item.snowdepth]) : [];
                
            default:
                return [];
        }
    };
    
    // Get additional series data (for charts like solar that have multiple series)
    const getAdditionalSeriesData = (rawData, chartType) => {
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
        getChartConfig,
        chartTypeConfigs
    };
})();

// Make available globally if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherCharts;
}