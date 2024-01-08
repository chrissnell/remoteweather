let humidityChart, temperatureChart, barometerChart, windspeedChart, winddirectionChart, rainfallChart, solarwattsChart, voltageChart;
let dataRefresherTimer, windDataRefresherTimer;

let tabList = {
    "day": {
        "graphDivSuffix": "24Hour",
        "elementIdVal": "tab-day",
        "spanHours": 24
    },
    "threedays": {
        "graphDivSuffix": "72Hours",
        "elementIdVal": "tab-threedays",
        "spanHours": 72
    },
    "week": {
        "graphDivSuffix": "Week",
        "elementIdVal": "tab-week",
        "spanHours": 168
    },
    "month": {
        "graphDivSuffix": "Month",
        "elementIdVal": "tab-month",
        "spanHours": 744
    },
    "year": {
        "graphDivSuffix": "Year",
        "elementIdVal": "tab-year",
        "spanHours": 8928
    }
}

let chartList = {
    "temperature":
    {
        "chart": temperatureChart,
        "divId": "temperatureChart",
        "title": "Outside Temperature",
        "yAxisLabel": "Degrees F",
        "chartType": "spline",
        "pointName": "outsideTempPoint",
        "initialSeries": [],
        "rendered": false

    },
    "humidity":
    {
        "chart": humidityChart,
        "divId": "humidityChart",
        "title": "Humidity",
        "yAxisLabel": "Percent",
        "chartType": "spline",
        "pointName": "outsideHumidityPoint",
        "initialSeries": [],
        "rendered": false
    },
    "barometer":
    {
        "chart": barometerChart,
        "divId": "barometerChart",
        "title": "Barometer",
        "yAxisLabel": "inches Hg",
        "chartType": "spline",
        "pointName": "barometerPoint",
        "initialSeries": [],
        "rendered": false
    },
    "windspeed":
    {
        "chart": windspeedChart,
        "divId": "windspeedChart",
        "title": "Wind Speed",
        "yAxisLabel": "MPH",
        "chartType": "spline",
        "pointName": "windSpeedPoint",
        "initialSeries": [],
        "rendered": false
    },
    "winddirection":
    {
        "chart": winddirectionChart,
        "divId": "winddirectionChart",
        "title": "Wind Direction",
        "yAxisLabel": "",
        "chartType": "vector",
        "pointName": "windDirectionVectorPoint",
        "initialSeries": [],
        "rendered": false
    },
    "rainfall":
    {
        "chart": rainfallChart,
        "divId": "rainfallChart",
        "title": "Rainfall",
        "yAxisLabel": "Inches",
        "chartType": "spline",
        "pointName": "rainfallPoint",
        "initialSeries": [],
        "rendered": false
    },
    "solarwatts":
    {
        "chart": solarwattsChart,
        "divId": "solarwattsChart",
        "title": "Solar Energy",
        "yAxisLabel": "Watts/m&sup2;",
        "chartType": "spline",
        "pointName": "solarWattsPoint",
        "initialSeries": [],
        "rendered": false
    },
    "voltage":
    {
        "chart": voltageChart,
        "divId": "voltageChart",
        "title": "Station Battery Voltage",
        "yAxisLabel": "Volts",
        "chartType": "spline",
        "pointName": "voltagePoint",
        "initialSeries": [],
        "rendered": false
    },
}

document.addEventListener('DOMContentLoaded', function () {
    fetchAndCreateCharts("day");

    for (let key in tabList) {
        var e = document.getElementById(tabList[key].elementIdVal);
        e.addEventListener("click", function (e) {
            clearCharts();
            fetchAndCreateCharts(key)
        })
    }
    refreshLiveReadings();
});

function clearCharts(element) {
    console.log("stopping data refresher---> ", dataRefresherTimer, windDataRefresherTimer)
    clearTimeout(dataRefresherTimer);
    clearTimeout(windDataRefresherTimer);
    console.log("clearing charts...")
    for (let key in chartList) {
        chartList[key].chart = {};
        chartList[key].initialSeries = [];
    }
}

async function fetchAndCreateCharts(tabKey) {
    const result = await fetch('/span/' + tabList[tabKey].spanHours + 'h?station=CSI');
    if (result.ok) {
        console.log(tabList[tabKey].spanHours, "of data fetched for", tabList[tabKey].graphDivSuffix)
        const data = await result.json();
        var count = 0;
        modVal = (data.length / 50).toFixed(0)
        data.forEach(function (item, index) {
            d = item.ts;
            chartList.temperature.initialSeries.push([d, item.otemp]);
            chartList.humidity.initialSeries.push([d, item.outhumidity]);
            chartList.barometer.initialSeries.push([d, item.bar]);
            chartList.windspeed.initialSeries.push([d, item.winds]);
            chartList.rainfall.initialSeries.push([d, item.rainincremental]);
            chartList.voltage.initialSeries.push([d, item.stationbatteryvoltage]);
            chartList.solarwatts.initialSeries.push([d, item.solarwatts]);
            if ((count % modVal) == 0) {
                chartList.winddirection.initialSeries.push([d, 5, item.winds, item.windd]);
            }
            count++;
        });
        for (let key in chartList) {
            chartElement = document.getElementById(chartList[key].divId + tabList[tabKey].graphDivSuffix);
            switch (chartList[key].chartType) {
                case "spline":
                    chartList[key].chart = singleVariableChart(key, chartElement, chartList[key].initialSeries, chartList[key].title, chartList[key].yAxisLabel);
                    break;
                case "vector":
                    chartList[key].chart = windVectorChart(key, chartElement, chartList[key].initialSeries, chartList[key].title);
            }
        }
        refreshData();
        refreshWindDirectionData();    
        console.log('DOM fully loaded and parsed');
    }
}

/**
 * Marks a chart as rendered and ready for periodic data refreshment
 */
async function markAsRendered(chart) {
    chartList[chart].rendered = true
}

/**
* Refresh data from the server, add it to the graph and set a timeout to request again
*/
async function refreshData() {
    console.log("Fetching latest data...")
    const result = await fetch('/latest?station=CSI');
    if (result.ok) {
        const data = await result.json();
        const ts = data.ts;
        var points = {
            "outsideTempPoint": [ts, data.otemp],
            "outsideHumidityPoint": [ts, data.outhumidity],
            "barometerPoint": [ts, data.bar],
            "windSpeedPoint": [ts, data.winds],
            // "windDirectionVectorPoint": [ts, 5, data.winds, data.windd],
            "voltagePoint": [ts, data.stationbatteryvoltage],
            "rainfallPoint": [ts, data.dayrain],
            "solarWattsPoint": [ts, data.solarwatts]
        }

        for (let key in chartList) {
            // The wind direction chart is a special one requiring its own updating
            // procedure, so we skip it here.
            if ((key != "winddirection") && (chartList[key].rendered == true)) {
                console.log("adding data point to", key)
                const series = chartList[key].chart.series[0]
                shift = series.data.length > 20;
                chartList[key].chart.series[0].addPoint(points[chartList[key].pointName]);
            }
        }

    }
    dataRefresherTimer = setTimeout(refreshData, 300000);
}

async function refreshWindDirectionData() {
    console.log("Fetching latest wind direction data...")
    const result = await fetch('/latest?station=CSI');
    if (result.ok) {
        const data = await result.json();
        const ts = data.ts;
        var windDirectionPoint = [ts, 5, data.winds, data.windd]
        if (chartList.winddirection.rendered == true) {
            console.log("adding data point to winddirection")
            const series = chartList.winddirection.chart.series[0]
            shift = series.data.length > 20;
            chartList.winddirection.chart.series[0].addPoint(windDirectionPoint);
        }

    }
    windDataRefresherTimer = setTimeout(refreshData, 1800000);
}


/**
 * Refreshes the live weather readings at the top of the page every 3 seconds
 */
async function refreshLiveReadings() {
    console.log("Fetching live data...")
    const result = await fetch('/latest?station=CSI');
    if (result.ok) {
        const data = await result.json();
        const ts = data.ts;

        document.getElementById("rdg-temperature").innerHTML = data.otemp.toFixed(2);
        document.getElementById("rdg-humidity").innerHTML = data.outhumidity.toFixed(0);
        document.getElementById("rdg-barometer").innerHTML = data.bar.toFixed(2);
        document.getElementById("rdg-windspeed").innerHTML = data.winds.toFixed(0);
        document.getElementById("rdg-winddir-cardinal").innerHTML = data.windcard;
        document.getElementById("rdg-winddir").style.transform = 'rotate(' + data.windd + 'deg)';
        document.getElementById("rdg-windandcardinal").innerHTML = data.winds.toFixed(0) + "<br>" + data.windcard;
        document.getElementById("rdg-rainfall").innerHTML = data.dayrain.toFixed(2);
    }
    setTimeout(refreshLiveReadings, 3000);
}



function windVectorChart(chartName, targetDiv, data, title) {
    Highcharts.Templating.helpers.date = function (value) {
        const d = new Date(value);
        return d.toLocaleString('en-US');
    };

    thisChart = Highcharts.chart(targetDiv, {
        legend: {
            enabled: false
        },
        chart: {
            displayErrors: true,
            style: {
                fontFamily: 'Inconsolata, monospace'
            },
            events: {
                load: function () {
                    markAsRendered(chartName);
                }
            }
        },
        tooltip: {
            pointFormat: '{date point.x}<br/>Speed: <b>{(point.length):.1f} MPH</b><br/>Direction: <b>{(point.direction):.0f}\u00B0</b><br/>'
        },
        title: {
            text: title
        },
        credits: {
            enabled: false
        },
        xAxis: {
            gridLineWidth: 1,
            type: 'datetime'
        },
        yAxis: {
            visible: false
        },
        time: {
            useUTC: false
        },
        series: [{
            type: 'vector',
            name: title,
            color: '#c1121f',
            data: data
        }]

    });
    return thisChart;
}

function singleVariableChart(chartName, targetDiv, data, title, yAxisTitle) {
    thisChart = Highcharts.chart(targetDiv, {
        legend: {
            enabled: false
        },
        chart: {
            displayErrors: true,
            type: 'spline',
            style: {
                fontFamily: 'Inconsolata, monospace'
            },
            events: {
                // load: requestData
                load: function () {
                    markAsRendered(chartName);
                }
            }
        },

        credits: {
            enabled: false
        },

        rangeSelector: {
            selected: 1
        },

        yAxis: {
            title: {
                text: yAxisTitle
            }
        },

        xAxis: {
            title: {
                enabled: true,
            },
            type: 'datetime',
        },

        title: {
            text: title
        },

        time: {
            useUTC: false
        },

        series: [{
            name: title,
            data: data,
            color: '#c1121f',
            tooltip: {
                valueDecimals: 2
            },
            states: {
                hover: {
                    lineWidth: 2
                }
            }
        }]
    });
    return thisChart;
}

