// Weather Portal DOM Management
// Handles all DOM updates and UI interactions

const PortalDOM = {
    // Element cache
    elements: {
        stationListContent: null,
        loadingIndicator: null,
        errorMessage: null,
        refreshStatus: null,
        dataDisplayButtons: null
    },

    // Initialize DOM elements
    init() {
        this.elements.stationListContent = document.getElementById('station-list-content');
        this.elements.loadingIndicator = document.getElementById('loading-indicator');
        this.elements.errorMessage = document.getElementById('error-message');
        this.elements.refreshStatus = document.getElementById('refresh-status');
        this.elements.dataDisplayButtons = document.querySelectorAll('.data-display-button');
    },

    // Update the station list in the sidebar
    updateStationList(stations, onStationClick) {
        const listContent = this.elements.stationListContent;
        listContent.innerHTML = '';
        
        stations.forEach(station => {
            const item = document.createElement('div');
            item.className = 'station-item';
            
            const name = document.createElement('div');
            name.className = 'station-name';
            name.textContent = station.name;
            
            const status = document.createElement('div');
            status.className = 'station-status';
            status.textContent = PortalUtils.getStatusText(station);
            
            const temp = document.createElement('div');
            temp.className = 'station-temp';
            temp.textContent = station.weather ? 
                PortalUtils.formatTemperature(station.weather.otemp) : '--';
            
            item.appendChild(name);
            item.appendChild(status);
            item.appendChild(temp);
            
            // Click handler to focus on station
            item.addEventListener('click', () => {
                onStationClick(station);
                // Update active state
                document.querySelectorAll('.station-item').forEach(i => i.classList.remove('active'));
                item.classList.add('active');
            });
            
            listContent.appendChild(item);
        });
    },

    // Show/hide loading indicator
    showLoading(show) {
        if (this.elements.loadingIndicator) {
            this.elements.loadingIndicator.style.display = show ? 'block' : 'none';
        }
    },

    // Show error message
    showError(message) {
        if (this.elements.errorMessage) {
            this.elements.errorMessage.querySelector('p').textContent = message;
            this.elements.errorMessage.style.display = 'block';
            
            // Auto-hide after 5 seconds
            setTimeout(() => {
                this.elements.errorMessage.style.display = 'none';
            }, 5000);
        }
    },

    // Update refresh status
    updateRefreshStatus(text) {
        if (this.elements.refreshStatus) {
            this.elements.refreshStatus.textContent = text;
        }
    },

    // Setup data display button event listeners
    setupDataDisplayButtons(onButtonClick) {
        this.elements.dataDisplayButtons.forEach(button => {
            button.addEventListener('click', (e) => {
                // Remove active class from all buttons
                this.elements.dataDisplayButtons.forEach(btn => btn.classList.remove('active'));
                
                // Add active class to clicked button
                button.classList.add('active');
                
                // Trigger callback with the selected data type
                onButtonClick(button.dataset.type);
            });
        });
    },

    // Set active data display button
    setActiveDataButton(dataType) {
        this.elements.dataDisplayButtons.forEach(button => {
            if (button.dataset.type === dataType) {
                button.classList.add('active');
            } else {
                button.classList.remove('active');
            }
        });
    }
};

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = PortalDOM;
}