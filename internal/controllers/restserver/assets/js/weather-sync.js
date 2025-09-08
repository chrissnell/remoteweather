// Synchronized tooltips for weather charts
const WeatherChartSync = (function() {
    'use strict';
    
    // Highcharts synchronized tooltip implementation
    function synchronizeTooltips(containerSelector) {
        const container = document.querySelector(containerSelector);
        if (!container) return;
        
        // Find all Highcharts charts within the container
        const charts = Highcharts.charts.filter(chart => 
            chart && chart.renderTo && container.contains(chart.renderTo)
        );
        
        if (charts.length < 2) return; // No need to sync single chart
        
        // Don't override the reset function globally as it breaks series states
        // Instead, handle tooltip hiding in the mouseleave event
        
        /**
         * Synchronize zooming through the setExtremes event handler
         */
        function syncExtremes(e) {
            const thisChart = this.chart;
            
            if (e.trigger !== 'syncExtremes') { // Prevent feedback loop
                Highcharts.each(charts, function(chart) {
                    if (chart !== thisChart) {
                        if (chart.xAxis[0].setExtremes) {
                            chart.xAxis[0].setExtremes(e.min, e.max, undefined, false, {
                                trigger: 'syncExtremes'
                            });
                        }
                    }
                });
            }
        }
        
        // Add synchronization to each chart
        charts.forEach((chart) => {
            const container = chart.renderTo;
            
            // Clean up previous listeners
            container.onmousemove = null;
            container.ontouchmove = null;
            container.onmouseleave = null;
            
            // Sync tooltips on mouse/touch move
            ['mousemove', 'touchmove'].forEach(eventType => {
                container.addEventListener(eventType, function(e) {
                    charts.forEach(chart2 => {
                        if (chart2 && chart2.pointer) {
                            const event = chart2.pointer.normalize(e);
                            
                            chart2.series.forEach(series => {
                                if (series && series.visible && series.points) {
                                    const point = series.searchPoint(event, true);
                                    if (point) {
                                        point.onMouseOver();
                                    } else if (series.type === 'vector' && series.points.length > 0) {
                                        // For vector charts with sparse data, find the nearest point
                                        const xPos = event.chartX;
                                        let nearestPoint = null;
                                        let minDistance = Infinity;
                                        
                                        series.points.forEach(p => {
                                            if (p && p.plotX !== undefined) {
                                                const distance = Math.abs(p.plotX - xPos);
                                                if (distance < minDistance) {
                                                    minDistance = distance;
                                                    nearestPoint = p;
                                                }
                                            }
                                        });
                                        
                                        if (nearestPoint && minDistance < 50) { // Within 50 pixels
                                            nearestPoint.onMouseOver();
                                        }
                                    }
                                }
                            });
                        }
                    });
                });
            });
            
            // Hide tooltips on mouse leave
            container.addEventListener('mouseleave', function() {
                charts.forEach(chart => {
                    if (chart.tooltip) {
                        chart.tooltip.hide();
                    }
                    if (chart.xAxis && chart.xAxis[0]) {
                        chart.xAxis[0].hideCrosshair();
                    }
                });
            });
            
            // Sync extremes for zooming
            if (chart.xAxis && chart.xAxis[0]) {
                chart.xAxis[0].update({
                    events: {
                        setExtremes: syncExtremes
                    }
                });
            }
        });
    }
    
    // Initialize synchronization for a specific time range
    function initializeForRange(range) {
        // Wait for charts to be fully rendered
        setTimeout(() => {
            const containerSelector = `#charts-${range}`;
            synchronizeTooltips(containerSelector);
        }, 100);
    }
    
    // Public API
    return {
        initializeForRange
    };
})();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = WeatherChartSync;
}